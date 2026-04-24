package main

import (
	"strconv"
)


func parsePageParam(rawpage string, standardLimit int) (offset int, limit int) {
	if rawpage == "all" {
		return 0, -1
	}
	page, err := strconv.Atoi(rawpage)

	// If there's any doubt about page number, start at page 1
	if err != nil {
		page = 1
	}
	offset = standardLimit * (page - 1)
	return offset, standardLimit
}
