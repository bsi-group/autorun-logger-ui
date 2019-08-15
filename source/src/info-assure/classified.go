package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	util "github.com/woanware/goutil"
)

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

		userID := getCookieInt64Value(c, "user_id")
		if userID == -1 {
			log.Println("Error retrieving user: invalid user ID")
			goToErrorPage(c, "Unable to perform classification")
			return
		}
		message = performAlertClassification(userID, ids, true)
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
