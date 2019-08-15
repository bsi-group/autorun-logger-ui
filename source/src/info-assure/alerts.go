package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	util "github.com/woanware/goutil"
)

// ##### Constants ############################################################

const SQL_ALERTS_UNCLASSIFIED string = `SELECT * 
	   FROM alert 
	   JOIN (SELECT alert.id FROM alert
  LEFT JOIN classification ON (classification.alert_id = alert.id)
      WHERE classification.id IS NULL
      LIMIT $1 
	 OFFSET $2) AS a ON a.id = alert.id
   ORDER BY alert.timestamp`

const SQL_ALERTS_UNCLASSIFIED_FILTERED string = `SELECT * 
	   FROM alert 
	   JOIN (SELECT alert.id FROM alert
  LEFT JOIN classification ON (classification.alert_id = alert.id)
      WHERE classification.id IS NULL AND alert.verified = $3
      LIMIT $1 
	 OFFSET $2) AS a ON a.id = alert.id
   ORDER BY alert.timestamp`

const SQL_ALERTS_CLASSIFIED string = `SELECT alert.*, users.username as classified_by, classification.timestamp as classified 
	   FROM alert
  LEFT JOIN classification ON (classification.alert_id = alert.id)
  	   JOIN users on (users.id = classification.user_id)
       JOIN (SELECT alert.id FROM alert
  LEFT JOIN classification ON (classification.alert_id = alert.id)
      WHERE classification.id IS NOT NULL
      LIMIT $1 
	 OFFSET $2) AS a ON a.id = alert.id
   ORDER BY alert.timestamp `

// ##### Methods ##############################################################

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

		userID := getCookieInt64Value(c, "user_id")
		if userID == -1 {
			log.Println("Error retrieving user: invalid user ID")
			goToErrorPage(c, "Unable to perform classification")
			return
		}

		message = performAlertClassification(userID, ids, false)
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
	} else {
		err = db.SQL(SQL_ALERTS_UNCLASSIFIED_FILTERED, numRecsPerPage+1, numRecsPerPage*currentPageNumber, verified).QueryStructs(&data)
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
func performAlertClassification(userID int64, data string, delete bool) string {

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

		b := db.InsertInto("classification").Columns("alert_id", "user_id", "timestamp")

		for _, id2 := range ids {
			b.Values(id2, userID, time.Now().UTC().Format(time.RFC3339))
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
