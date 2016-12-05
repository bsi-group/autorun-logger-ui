package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	util "github.com/woanware/goutil"
	"html/template"
	"net/http"
	"path"
	"strings"
	"time"
)

const SQL_ALERTS_UNCLASSIFIED string = `SELECT * FROM alert JOIN (SELECT alert.id FROM alert
LEFT JOIN classification ON (classification.alert_id = alert.id)
WHERE classification.id IS NULL ORDER BY alert.timestamp LIMIT $1 OFFSET $2) AS a ON a.id = alert.id`

const SQL_ALERTS_UNCLASSIFIED_FILTERED string = `SELECT * FROM alert JOIN (SELECT alert.id FROM alert
LEFT JOIN classification ON (classification.alert_id = alert.id)
WHERE classification.id IS NULL AND alert.verified = $3 ORDER BY alert.timestamp LIMIT $1 OFFSET $2) AS a ON a.id = alert.id`

const SQL_ALERTS_CLASSIFIED string = `SELECT alert.*, 
classification.user_name as classified_by, classification.timestamp as classified FROM alert
LEFT JOIN classification ON (classification.alert_id = alert.id)
JOIN (SELECT alert.id FROM alert
LEFT JOIN classification ON (classification.alert_id = alert.id)
WHERE classification.id IS NOT NULL ORDER BY alert.timestamp LIMIT $1 OFFSET $2) AS a ON a.id = alert.id`

//
func routeIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index", gin.H{})
}

//
func routeAlerts(c *gin.Context) {

	numRecsPerPage, successful := processIntParameter(c.PostForm("num_recs_per_page"))
	if successful == false {
		numRecsPerPage = 10
	}

	verified, successful := processIntParameter(c.PostForm("verified"))
	if successful == false {
		numRecsPerPage = 10
	}

	// Appears to be the first request to send the initial set of data
	if verified != VERIFIED_ALL &&
		verified != VERIFIED_TRUE &&
		verified != VERIFIED_FALSE &&
		verified != VERIFIED_MS {

		loadAlertData(c, 0, numRecsPerPage, VERIFIED_ALL, "")
		return
	}

	mode, hasMode := c.GetPostForm("mode")

	// Appears to be the first request to send the initial set of data
	if (mode != "first" &&
		mode != "next" &&
		mode != "previous" &&
		mode != "classify") || hasMode == false {

		loadAlertData(c, 0, numRecsPerPage, verified, "")
		return
	}

	currentPageNumber := processCurrentPageNumber(c.PostForm("current_page_num"), mode)

	message := ""
	if mode == "classify" {
		// Ensure that we have some alert ID's to classify
		ids, idsExist := c.GetPostForm("ids")
		if idsExist == false {

			loadAlertData(c, currentPageNumber, numRecsPerPage, verified, "No alert's supplied for classification")
			return
		}

		user := c.MustGet(gin.AuthUserKey).(string)
		message = performAlertClassification(user, ids, false)
	}

	loadAlertData(c, currentPageNumber, numRecsPerPage, verified, message)
}

//
func loadAlertData(
	c *gin.Context,
	currentPageNumber int,
	numRecsPerPage int,
	verified int, error string) {

	errored, noMoreRecords, data := getAlerts(numRecsPerPage, currentPageNumber, verified)
	if errored == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	c.HTML(http.StatusOK, "alerts", gin.H{
		"current_page_num":  currentPageNumber,
		"num_recs_per_page": numRecsPerPage,
		"no_more_records":   noMoreRecords,
		"verified":          verified,
		"data":              data,
		"error":             error,
	})
}

//
func getAlerts(numRecsPerPage int, currentPageNumber int, verified int) (bool, bool, []*Alert) {

	var data []*Alert
	var err error

	if verified == VERIFIED_ALL {
		err = db.SQL(SQL_ALERTS_UNCLASSIFIED, numRecsPerPage+1, numRecsPerPage*currentPageNumber).QueryStructs(&data)

		// err = db.
		// 	Select(`id, instance, domain, host, "timestamp", autorun_id, location, item_name,
		// 			enabled, profile, launch_string, description, company, signer, version_number,
		// 			file_path, file_name, file_directory, "time", sha256, md5, text, linked`).
		// 	From("alert").
		// 	OrderBy("timestamp DESC").
		// 	Offset(uint64(numRecsPerPage * currentPageNumber)).
		// 	Limit(uint64(numRecsPerPage + 1)).
		// 	QueryStructs(&data)
	} else {
		err = db.SQL(SQL_ALERTS_UNCLASSIFIED_FILTERED, numRecsPerPage+1, numRecsPerPage*currentPageNumber, verified).QueryStructs(&data)

		// err = db.
		// 	Select(`id, instance, domain, host, "timestamp", autorun_id, location, item_name,
		// 			enabled, profile, launch_string, description, company, signer, version_number,
		// 			file_path, file_name, file_directory, "time", sha256, md5, text, linked`).
		// 	From("alert").
		// 	Where("verified = $1", verified).
		// 	OrderBy("timestamp DESC").
		// 	Offset(uint64(numRecsPerPage * currentPageNumber)).
		// 	Limit(uint64(numRecsPerPage + 1)).
		// 	QueryStructs(&data)
	}

	if err != nil {
		logger.Errorf("Error querying for alerts: %v", err)
		return true, false, data
	}

	// Perform some cleaning of the data, so that it displays better in the HTML
	for _, v := range data {
		v.LocationStr = template.HTML("<td class=\"poppy\" data-variation=\"basic\" data-content=\"" + v.Location + "\">" + splitRegKey(v.Location) + "</td>")
		v.UtcTimeStr = v.UtcTime.Format("15:04:05 02/01/2006")
		v.TextStr = template.HTML(v.Text)
		v.LinkedStr = template.HTML(v.Linked)

		if len(v.Linked) > 0 {
			v.LinkedColumn = template.HTML("<td style=\"text-align:center\"><a href=\"#\" class=\"togglerLinked\" other-data=\"" + util.ConvertInt64ToString(v.Id) + "\"><i class=\"checkmark icon\"></i></a></td>")
		} else {
			v.LinkedColumn = template.HTML("<td></td>")
		}
	}

	noMoreRecords := false
	if len(data) < numRecsPerPage+1 {
		noMoreRecords = true
	} else {
		// Remove the last item in the slice/array
		data = data[:len(data)-1]
	}

	return false, noMoreRecords, data
}

//
func performAlertClassification(userName string, data string, delete bool) string {

	ids := strings.Split(data, ",")
	for _, id := range ids {
		if util.IsNumber(id) == false {
			return "Error performing classification"
		}
	}

	tx, err := db.Begin()
	if err != nil {
		logger.Errorf("Error starting classication transaction: %v", err)
		return "Error performing classification"
	}

	errorOccurred := false

	if delete == true {

		for _, id1 := range ids {
			_, err = db.
				DeleteFrom("classification").
				Where("alert_id = $1", id1).
				Exec()

			if err != nil {
				errorOccurred = true
				logger.Errorf("Error deleting classification: %v (Alert: %d)", err, id1)
				break
			}
		}
	} else {

		b := db.InsertInto("classification").Columns("alert_id", "user_name", "timestamp")

		for _, id2 := range ids {
			b.Values(id2, userName, time.Now().UTC().Format(time.RFC3339))
		}

		_, err = b.Exec()
		if err != nil {
			errorOccurred = true
			logger.Errorf("Error inserting classification: %v", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Errorf("Error commiting classication transaction: %v", err)
	}

	if errorOccurred == true {
		return "Error performing classification. Refresh the page"
	}

	return ""
}

//
func routeClassified(c *gin.Context) {

	numRecsPerPage, successful := processIntParameter(c.PostForm("num_recs_per_page"))
	if successful == false {
		numRecsPerPage = 10
	}

	mode, hasMode := c.GetPostForm("mode")

	// Appears to be the first request to send the initial set of data
	if (mode != "first" &&
		mode != "next" &&
		mode != "previous" &&
		mode != "unclassify") || hasMode == false {

		loadClassifiedAlertData(c, 0, numRecsPerPage, "")
		return
	}

	currentPageNumber := processCurrentPageNumber(c.PostForm("current_page_num"), mode)

	message := ""
	if mode == "unclassify" {
		// Ensure that we have some alert ID's to unclassify
		ids, idsExist := c.GetPostForm("ids")
		if idsExist == false {

			loadClassifiedAlertData(c, currentPageNumber, numRecsPerPage, "No alert's supplied for unclassification")
			return
		}

		user := c.MustGet(gin.AuthUserKey).(string)
		message = performAlertClassification(user, ids, true)
	}

	loadClassifiedAlertData(c, currentPageNumber, numRecsPerPage, message)
}

//
func loadClassifiedAlertData(
	c *gin.Context,
	currentPageNumber int,
	numRecsPerPage int,
	error string) {

	errored, noMoreRecords, data := getClassifiedAlerts(numRecsPerPage, currentPageNumber)
	if errored == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	c.HTML(http.StatusOK, "classified", gin.H{
		"current_page_num":  currentPageNumber,
		"num_recs_per_page": numRecsPerPage,
		"no_more_records":   noMoreRecords,
		"data":              data,
		"error":             error,
	})
}

//
func getClassifiedAlerts(numRecsPerPage int, currentPageNumber int) (bool, bool, []*ClassifiedAlert) {

	var data []*ClassifiedAlert
	var err error

	err = db.SQL(SQL_ALERTS_CLASSIFIED, numRecsPerPage+1, numRecsPerPage*currentPageNumber).QueryStructs(&data)

	if err != nil {
		logger.Errorf("Error querying for unclassified alerts: %v", err)
		return true, false, data
	}

	// Perform some cleaning of the data, so that it displays better in the HTML
	for _, v := range data {
		v.LocationStr = template.HTML("<td class=\"poppy\" data-variation=\"basic\" data-content=\"" + v.Location + "\">" + splitRegKey(v.Location) + "</td>")
		v.UtcTimeStr = v.UtcTime.Format("15:04:05 02/01/2006")
		v.TextStr = template.HTML(v.Text)
		v.LinkedStr = template.HTML(v.Linked)

		if len(v.Linked) > 0 {
			v.LinkedColumn = template.HTML("<td style=\"text-align:center\"><a href=\"#\" class=\"togglerLinked\" other-data=\"" + util.ConvertInt64ToString(v.Id) + "\"><i class=\"checkmark icon\"></i></a></td>")
		} else {
			v.LinkedColumn = template.HTML("<td></td>")
		}
	}

	noMoreRecords := false
	if len(data) < numRecsPerPage+1 {
		noMoreRecords = true
	} else {
		// Remove the last item in the slice/array
		data = data[:len(data)-1]
	}

	return false, noMoreRecords, data
}

//
func routeSingleHost(c *gin.Context) {

	host := c.PostForm("host")

	if len(host) == 0 {
		c.HTML(http.StatusOK, "single_host", gin.H{
			"has_data": false,
			"host":     "",
			"data":     nil,
		})
		return
	}

	errored, data := getAutorunsForSingleHost(host)
	if errored == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	hasData := true
	if len(data) == 0 {
		hasData = false
	}

	c.HTML(http.StatusOK, "single_host", gin.H{
		"has_data": hasData,
		"host":     host,
		"data":     data,
	})
}

func getAutorunsForSingleHost(host string) (bool, []*Alert) {
	var i Instance
	var data []*Alert

	err := db.
		Select(`id, domain, host`).
		From("instance").
		Where("LOWER(instance.host) = LOWER($1)", host).
		Limit(1).
		OrderBy("timestamp DESC").
		QueryStruct(&i)

	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") == true {
			return false, data
		}

		logger.Errorf("Error querying for a hosts instance: %v", err)
		return true, data
	}

	err = db.
		Select(`id, location, item_name, enabled, profile, launch_string, description, company, signer, version_number, file_path, file_name, file_directory, time, sha256, md5`).
		From("current_autoruns").
		Where("instance = $1", i.Id).
		OrderBy("location, item_name").
		QueryStructs(&data)

	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") == true {
			return false, data
		}

		logger.Errorf("Error querying for a hosts instance: %v", err)
		return true, data
	}

	// Perform some cleaning of the data, so that it displays better in the HTML
	for _, v := range data {
		v.LocationStr = template.HTML("<td class=\"poppy\" data-variation=\"basic\" data-content=\"" + v.Location + "\">" + splitRegKey(v.Location) + "</td>")
		v.TimeStr = v.Time.Format("15:04:05 02/01/2006")
		v.TextStr = template.HTML(v.Text)
		v.LinkedStr = template.HTML(v.Linked)
	}

	return false, data
}

//
func routeSearch(c *gin.Context) {

	currentPageNumber := 0

	numRecsPerPage, successful := processIntParameter(c.PostForm("num_recs_per_page"))
	if successful == false {
		numRecsPerPage = 10
	}

	mode, hasMode := c.GetPostForm("mode")

	// Appears to be the first request to send the initial set of data
	if (mode != "first" &&
		mode != "next" &&
		mode != "previous") || hasMode == false {

		loadSearchData(c, 0, 0, "", currentPageNumber, numRecsPerPage)
		return
	}

	searchType, successful := processIntParameter(c.PostForm("search_type"))
	if successful == false {
		c.String(http.StatusInternalServerError, "")
		return
	}

	dataType, successful := processIntParameter(c.PostForm("data_type"))
	if successful == false {
		c.String(http.StatusInternalServerError, "")
		return
	}

	searchValue := c.PostForm("search_value")
	if len(searchValue) == 0 {
		c.String(http.StatusInternalServerError, "")
		return
	}

	currentPageNumber = processCurrentPageNumber(c.PostForm("current_page_num"), mode)

	loadSearchData(c, dataType, searchType, searchValue, currentPageNumber, numRecsPerPage)
}

//
func loadSearchData(
	c *gin.Context,
	dataType int,
	searchType int,
	searchValue string,
	currentPageNumber int,
	numRecsPerPage int) {

	if len(searchValue) == 0 || (searchType < 1 || searchType > 10) || (dataType < 1 || dataType > 2) {
		c.HTML(http.StatusOK, "search", gin.H{
			"current_page_num":  currentPageNumber,
			"num_recs_per_page": numRecsPerPage,
			"no_more_records":   true,
			"data":              nil,
			"has_data":          false,
			"data_type":         0,
			"search_type":       0,
			"search_value":      searchValue,
		})
		return
	}

	errored, noMoreRecords, data := getSearch(dataType, searchType, searchValue, numRecsPerPage, currentPageNumber)
	if errored == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	hasData := true
	if len(data) == 0 {
		hasData = false
	}

	c.HTML(http.StatusOK, "search", gin.H{
		"current_page_num":  currentPageNumber,
		"num_recs_per_page": numRecsPerPage,
		"no_more_records":   noMoreRecords,
		"data":              data,
		"has_data":          hasData,
		"data_type":         dataType,
		"search_type":       searchType,
		"search_value":      searchValue,
	})
}

func getSearch(
	dataType int,
	searchType int,
	searchValue string,
	numRecsPerPage int,
	currentPageNumber int) (bool, bool, []*Alert) {

	where := ""
	switch searchType {
	case SEARCH_TYPE_FILE_PATH:
		where = "LOWER(d.file_path) LIKE $1"
	case SEARCH_TYPE_LAUNCH_STRING:
		where = "LOWER(d.launch_string) LIKE $1"
	case SEARCH_TYPE_LOCATION:
		where = "LOWER(d.location) LIKE $1"
	case SEARCH_TYPE_ITEM_NAME:
		where = "LOWER(d.item_name) LIKE $1"
	case SEARCH_TYPE_PROFILE:
		where = "LOWER(d.profile) LIKE $1"
	case SEARCH_TYPE_DESCRIPTION:
		where = "LOWER(d.description) LIKE $1"
	case SEARCH_TYPE_COMPANY:
		where = "LOWER(d.company) LIKE $1"
	case SEARCH_TYPE_SIGNER:
		where = "LOWER(d.signer) LIKE $1"
	case SEARCH_TYPE_SHA256:
		where = "LOWER(d.sha256) LIKE $1"
	case SEARCH_TYPE_MD5:
		where = "LOWER(d.md5) LIKE $1"
	}

	// We use an Alert struct rather than Autorun since it has the Domain/Host fields we need
	var data []*Alert

	selectSql := `i.domain, i.host, d.id, d.location, d.item_name, d.enabled,
		d.profile, d.launch_string, d.description, d.company, d.signer, d.version_number, d.file_path,
		d.file_name, d.file_directory, d.time, d.sha256, d.md5`
	fromSql := `current_autoruns d JOIN instance i on (d.instance = i.id)`

	if dataType == DATA_TYPE_ALERTS {
		selectSql = `d.domain, d.host, d.id, d.location, d.item_name, d.enabled,
			d.profile, d.launch_string, d.description, d.company, d.signer, d.version_number, d.file_path,
			d.file_name, d.file_directory, d.time, d.sha256, d.md5`
		fromSql = `alert d`
	}

	err := db.
		Select(selectSql).
		From(fromSql).
		OrderBy("d.time DESC").
		Offset(uint64(numRecsPerPage*currentPageNumber)).
		Limit(uint64(numRecsPerPage+1)).
		Where(where, "%"+strings.ToLower(searchValue)+"%").
		QueryStructs(&data)

	if err != nil {
		logger.Errorf("Error querying for search: %v", err)
		return true, false, data
	}

	// Perform some cleaning of the data, so that it displays better in the HTML
	for _, v := range data {
		v.UtcTimeStr = v.Time.Format("15:04:05 02/01/2006")
		v.TextStr = template.HTML(fmt.Sprintf(
			`<strong>File Path:</strong> %s<br>
			<strong>Launch String:</strong> %s<br>
			<strong>Enabled:</strong> %t<br>
			<strong>Description:</strong> %s<br>
			<strong>Company:</strong> %s<br>
			<strong>Signer:</strong> %s<br>
			<strong>Version:</strong> %s<br>
			<strong>Time:</strong> %s<br>
			<strong>SHA256:</strong> %s<br>
			<strong>MD5:</strong> %s<br>`,
			v.FilePath, v.LaunchString, v.Enabled, v.Description, v.Company, v.Signer, v.VersionNumber, v.Time, v.Sha256, v.Md5))
	}

	noMoreRecords := false
	if len(data) < numRecsPerPage+1 {
		noMoreRecords = true
	} else {
		// Remove the last item in the slice/array
		data = data[:len(data)-1]
	}

	return false, noMoreRecords, data
}

//
func routeExport(c *gin.Context) {

	exportType := 0

	temp := c.PostForm("export_type")
	if len(temp) > 0 {
		if util.IsNumber(temp) == true {
			exportType = util.ConvertStringToInt(temp)
		}
	}

	if exportType == 0 {
		c.HTML(http.StatusOK, "export", gin.H{
			"has_data":    false,
			"export_type": 0,
			"data":        nil,
		})
		return
	}

	errored, data := getExports(exportType)
	if errored == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	hasData := true
	if len(data) == 0 {
		hasData = false
	}

	c.HTML(http.StatusOK, "export", gin.H{
		"has_data":    hasData,
		"export_type": exportType,
		"data":        data,
	})
}

//
func getExports(exportType int) (bool, []*Export) {

	var data []*Export

	err := db.
		Select(`*`).
		From("export").
		Where("data_type = $1", exportType).
		Limit(10).
		OrderBy("updated").
		QueryStructs(&data)

	if err != nil {
		logger.Errorf("Error querying for exports: %v (%d)", err, exportType)
		return true, data
	}

	// Perform some cleaning of the data, so that it displays better in the HTML
	for _, v := range data {
		v.OtherData = template.HTML(`<a href="/export/` + util.ConvertInt64ToString(v.Id) + `">` + v.Updated.Format("15:04:05 02/01/2006") + `</a>`)
	}

	return false, data
}

//
func routeExportData(c *gin.Context) {

	id, successful := processInt64Parameter(c.Param("id"))
	if successful == false {
		c.String(http.StatusInternalServerError, "")
		return
	}

	if id < 1 {
		c.String(http.StatusInternalServerError, "")
		return
	}

	errored, export := getExport(id)

	if errored == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	// Load file contents
	if util.DoesFileExist(path.Join(config.ExportDir, export.FileName)) == false {
		logger.Errorf("Export file does not exist: %s", export.FileName)
		c.String(http.StatusInternalServerError, "")
		return
	}

	// Return file contents as download
	data, err := util.ReadTextFromFile(path.Join(config.ExportDir, export.FileName))
	if err != nil {
		logger.Errorf("Error reading export file: %v (%s)", err, export.FileName)
		c.String(http.StatusInternalServerError, "")
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\""+export.FileName)
	c.Data(http.StatusOK, "text/csv", []byte(data))
}

//
func getExport(id int64) (bool, Export) {

	var e Export

	err := db.
		Select(`id, data_type, file_name, updated`).
		From("export").
		Where("id = $1", id).
		QueryStruct(&e)

	if err != nil {
		logger.Errorf("Error querying for export: %v", err)
		return true, e
	}

	return false, e
}
