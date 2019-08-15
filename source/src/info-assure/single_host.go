package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	util "github.com/woanware/goutil"
)

//
func routeSingleHost(c *gin.Context) {

	searchHost := c.PostForm("search_host")
	host := c.PostForm("host")
	instance := c.PostForm("instance")

	if len(searchHost) == 0 && len(host) == 0 && len(instance) == 0 {
		c.HTML(http.StatusOK, "single_host", gin.H{
			"search_host": "",
			"data":        nil,
			"hosts":       nil,
		})
		return
	}

	// User is searching for a host
	if len(searchHost) > 0 {
		hosts, err := getHosts(searchHost)
		if err != nil {
			fmt.Printf("Erro retrieving hosts for single host: %v (%v)", err, searchHost)
			c.String(http.StatusInternalServerError, "")
			return
		}

		if len(hosts) > 1 {
			c.HTML(http.StatusOK, "single_host", gin.H{
				"data":        nil,
				"search_host": "",
				"hosts":       hosts,
			})
			return
		}

		host = hosts[0].Host
	}

	// User has selected a host to drill down
	if len(host) > 0 {

		// Either retrieve the instance ID for the host or
		// convert the string value returned from the page
		var instanceID int64
		if len(instance) == 0 {
			instanceID = getInstanceFromHost(host)
		} else {
			instanceID = util.ConvertStringToInt64(instance)
		}

		fmt.Println(host)
		fmt.Println(instanceID)

		if instanceID == -1 {
			c.String(http.StatusInternalServerError, "")
			return
		}

		mode, hasMode := c.GetPostForm("mode")
		numRecsPerPage, successful := processIntParameter(c.PostForm("num_recs_per_page"))
		if successful == false {
			numRecsPerPage = 10
		}

		// Appears to be the first request to send the initial set of data
		if (mode != "first" &&
			mode != "next" &&
			mode != "previous" &&
			mode != "export") || hasMode == false {

			loadSingleHostAutorunsData(c, host, instanceID, 0, numRecsPerPage)
			return
		}

		// Export single host's autorun data
		if hasMode == true && mode == "export" {
			exportSingleHostAutoruns(c, instanceID, host)
			return
		}

		currentPageNumber := processCurrentPageNumber(c.PostForm("current_page_num"), mode)

		loadSingleHostAutorunsData(c, host, instanceID, currentPageNumber, numRecsPerPage)
		return
	}

	// Default action
	c.HTML(http.StatusOK, "single_host", gin.H{
		"search_host": searchHost,
		"data":        nil,
		"hosts":       nil,
	})
}

// loadSingleHostAutorunsData performs the data retrieval and processing for a single hosts autoruns data
func loadSingleHostAutorunsData(
	c *gin.Context,
	host string,
	instance int64,
	currentPageNumber int,
	numRecsPerPage int) {

	errored, noMoreRecords, data := getPagedSingleHostAutoruns(instance, currentPageNumber, numRecsPerPage)
	if errored == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	fmt.Printf("Data: %v", data)

	c.HTML(http.StatusOK, "single_host_data", gin.H{
		"search_host":       host,
		"instance":          instance,
		"data":              data,
		"current_page_num":  currentPageNumber,
		"num_recs_per_page": numRecsPerPage,
		"no_more_records":   noMoreRecords,
	})
}

// getInstanceFromHost returns the ID of an instance that relates to the host specified
func getInstanceFromHost(host string) int64 {

	var i Instance
	err := db.
		Select(`id, domain, host`).
		From("instance").
		Where("LOWER(instance.host) = LOWER($1)", host).
		Limit(1).
		OrderBy("timestamp DESC").
		QueryStruct(&i)

	if err != nil {
		logger.Errorf("Error querying for instance from host: %v (%s)", err, host)
		return -1
	}

	return i.Id
}

// getPagedSingleHostAutoruns returns a paged set of autoruns, specific to a host
func getPagedSingleHostAutoruns(instance int64, currentPageNumber int, numRecsPerPage int) (errored bool, noMoreRecords bool, data []*Autorun) {

	errored = false

	err := db.
		Select(`id, location, item_name, enabled, profile, launch_string, description, company, signer, version_number, file_path, file_name, file_directory, time, sha256, md5, text`).
		From("current_autoruns").
		Where("instance = $1", instance).
		Limit(uint64(numRecsPerPage) + 1).
		Offset(uint64(numRecsPerPage) * uint64(currentPageNumber)).
		OrderBy("location, item_name").
		QueryStructs(&data)

	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") == true {
			noMoreRecords = true
			return
		}

		errored = true
		noMoreRecords = true

		logger.Errorf("Error querying for single host data: %v (Instance: %d)", err, instance)
		return
	}

	// Perform some cleaning of the data, so that it displays better in the HTML
	for _, d := range data {
		d.LocationStr = template.HTML("<td class=\"poppy\" data-variation=\"basic\" data-content=\"" + d.Location + "\">" + splitRegKey(d.Location) + "</td>")
		d.TextStr = template.HTML(d.Text)
		d.TimeStr = d.Time.Format("15:04:05 02/01/2006")
	}

	if len(data) < numRecsPerPage+1 {
		noMoreRecords = true
	} else {
		// Remove the last item in the slice/array
		data = data[:len(data)-1]
	}

	return
}

// getSingleHostAutoruns returns a set of autoruns, specific to a host
func getSingleHostAutoruns(instance int64) (data []*Autorun, errored bool) {

	errored = false

	err := db.
		Select(`id, location, item_name, enabled, profile, launch_string, description, company, signer, version_number, file_path, file_name, file_directory, time, sha256, md5`).
		From("current_autoruns").
		Where("instance = $1", instance).
		OrderBy("location, item_name").
		QueryStructs(&data)

	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") == true {
			return
		}

		errored = true

		logger.Errorf("Error querying for single host export data: %v (Instance: %d)", err, instance)
		return
	}

	return
}

// exportSingleHostAutoruns is the core function to extract
// the autorun data for a single host and return it as a CSV download
func exportSingleHostAutoruns(c *gin.Context, instance int64, host string) {

	data, err := getSingleHostAutoruns(instance)
	if err == true {
		c.String(http.StatusInternalServerError, "")
		return
	}

	if len(data) == 0 {
		// Should never reach here
		c.String(http.StatusInternalServerError, "")
		return
	}

	buffer := generateSingleHostAutorunsCsv(host, data)

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+host+".csv")
	c.Data(http.StatusOK, "application/octet-stream", buffer)
}

// generateSingleHostAutorunsCsv returns the CSV content as a byte slice
func generateSingleHostAutorunsCsv(host string, autoruns []*Autorun) []byte {

	buffer := new(bytes.Buffer)
	cw := csv.NewWriter(buffer)
	cw.Write([]string{"LOCATION", "NAME", "ENABLED", "PROFILE", "LAUNCH_STRING", "DESCRIPTION", "COMPANY", "SIGNER", "VERSION", "PATH", "TIMESTAMP", "SHA256", "MD5"})

	for _, a := range autoruns {
		cw.Write([]string{
			a.Location,
			a.ItemName,
			strconv.FormatBool(a.Enabled),
			a.Profile,
			a.LaunchString,
			a.Description,
			a.Company,
			a.Signer,
			a.VersionNumber,
			a.FilePath,
			a.Time.String(),
			a.Sha256,
			a.Md5})
	}

	return buffer.Bytes()
}
