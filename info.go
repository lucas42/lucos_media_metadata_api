package main

import (
	"net/http"
)

type InfoStruct struct {
	System  string `json:"system"`
	Checks  map[string]Check `json:"checks"`
	Metrics map[string]string `json:"metrics"`
}

type Check struct {
	TechDetail string `json:"techDetail"`
	OK bool`json:"ok"`
	Debug string `json:"debug,omitempty"`
}

/** 
 * A controller for serving /_info
 */
func (store Datastore) InfoController(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		info := InfoStruct{System: "lucos_media_metadata_api", Metrics: map[string]string{}}

		dbCheck := Check{TechDetail:"Does basic SELECT query from database"}
		var result int
		err := store.DB.Get(&result, "SELECT 1")
		if err != nil {
			dbCheck.OK = false
			dbCheck.Debug = err.Error()
		} else {
			dbCheck.OK = true
		}
		info.Checks = map[string]Check{
			"db": dbCheck,
		}
		writeJSONResponse(w, info, nil)
	} else {
		MethodNotAllowed(w, []string{"GET"})
	}
}