package main

import (
	util "github.com/woanware/goutil"
	"strings"
)

func splitRegKey(data string) string {
	parts := strings.Split(data, "\\")
	if len(parts) == 1 {
		return data
	}

	return parts[0] + "..." + parts[len(parts) -1]
}

//
func processIntParameter(data string) (int, bool) {
	if len(data) > 0 {
		if util.IsNumber(data) == true {
			return util.ConvertStringToInt(data), true
		}
	}

	return -1, false
}

//
func processInt64Parameter(data string) (int64, bool) {
	if len(data) > 0 {
		if util.IsNumber(data) == true {
			return util.ConvertStringToInt64(data), true
		}
	}

	return -1, false
}

//
func processCurrentPageNumber(data string, mode string) (int) {
	if len(data) == 0 {
		return 0
	}

	if util.IsNumber(data) == false {
		return 0
	}

	currentPageNumber := util.ConvertStringToInt(data)

	if mode == "first"{
		return 0
	}

	if mode == "next" {
		currentPageNumber += 1
		return currentPageNumber
	}

	if mode == "previous" {
		currentPageNumber -= 1
		return currentPageNumber
	}

	if currentPageNumber < 0 {
		return 0
	}

	return 0
}