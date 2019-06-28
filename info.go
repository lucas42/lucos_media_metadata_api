package main

import (
	"net/http"
)

type InfoStruct struct {
	System  string `json:"system"`
	Checks  map[string]string `json:"checks"`
	Metrics map[string]string `json:"metrics"`
}

/** 
 * A controller for serving /_info
 */
func (store Datastore) InfoController(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		info := InfoStruct{System: "lucos_media_metadata_api", Checks: map[string]string{}, Metrics: map[string]string{}}
		writeJSONResponse(w, info, nil)
	} else {
		MethodNotAllowed(w, []string{"GET"})
	}
}