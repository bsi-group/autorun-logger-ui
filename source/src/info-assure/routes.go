package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	util "github.com/woanware/goutil"
)

//
func routeIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index", gin.H{})
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
<strong>Launch String:</strong> %s
<strong>Enabled:</strong> %t
<strong>Description:</strong> %s
<strong>Company:</strong> %s
<strong>Signer:</strong> %s
<strong>Version:</strong> %s
<strong>Time:</strong> %s
<strong>SHA256:</strong> %s
<strong>MD5:</strong> %s`,
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

// //
// func routeLogon(c *gin.Context) {

// 	// domain := "woanware.eu.auth0.com"
// 	// aud := ""

// 	// conf := &oauth2.Config{
// 	// 	ClientID:     "hhJm3c170y6rfmbzbstS0Q46Eriza5CY",
// 	// 	ClientSecret: "eBBD4h2mU_JKQqpWdgG1uCz587-wfTON-r9UWarjFZPBgvYHWYXdqxdPYUAUIAVz",
// 	// 	RedirectURL:  "http://0.0.0.0:8080/callback",
// 	// 	Scopes:       []string{"openid", "profile"},
// 	// 	Endpoint: oauth2.Endpoint{
// 	// 		AuthURL:  "https://" + domain + "/authorize",
// 	// 		TokenURL: "https://" + domain + "/oauth/token",
// 	// 	},
// 	// }

// 	// if aud == "" {
// 	// 	aud = "https://" + domain + "/userinfo"
// 	// }

// 	// // Generate random state
// 	// b := make([]byte, 32)
// 	// rand.Read(b)
// 	// state := base64.StdEncoding.EncodeToString(b)

// 	// session, err := sessionStore.Get(c.Request, "state")
// 	// if err != nil {
// 	// 	log.Printf("Error getting user session: %v\n", err)
// 	// 	goToErrorPage(c, "Unable to perform login")
// 	// 	return
// 	// }
// 	// session.Values["state"] = state
// 	// err = session.Save(c.Request, c.Writer)
// 	// if err != nil {
// 	// 	log.Printf("Error saving session state: %v\n", err)
// 	// 	goToErrorPage(c, "Unable to perform login")
// 	// 	return
// 	// }

// 	// audience := oauth2.SetAuthURLParam("audience", aud)
// 	// url := conf.AuthCodeURL(state, audience)

// 	// c.Redirect(http.StatusTemporaryRedirect, url)
// }
