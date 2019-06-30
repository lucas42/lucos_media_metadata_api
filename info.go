package main

import (
	"net/http"
)

type InfoStruct struct {
	System  string            `json:"system"`
	Checks  map[string]Check  `json:"checks"`
	Metrics map[string]Metric `json:"metrics"`
}

type Check struct {
	TechDetail string `json:"techDetail"`
	OK         bool   `json:"ok"`
	Debug      string `json:"debug,omitempty"`
}

type Metric struct {
	TechDetail string `json:"techDetail"`
	Value      int    `json:"value"`
}

/** 
 * A controller for serving /_info
 */
func (store Datastore) InfoController(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		info := InfoStruct{System: "lucos_media_metadata_api"}

		dbCheck := Check{TechDetail:"Does basic SELECT query from database"}
		trackCount := Metric{TechDetail:"Number of tracks in database"}
		err := store.DB.Get(&trackCount.Value, "SELECT COUNT(*) FROM track")
		if err != nil {
			dbCheck.OK = false
			dbCheck.Debug = err.Error()
		} else {
			dbCheck.OK = true
		}

		info.Checks = map[string]Check{
			"db": dbCheck,
		}
		info.Metrics = map[string]Metric{
			"trackCount": trackCount,
		}
		writeJSONResponse(w, info, nil)
	} else {
		MethodNotAllowed(w, []string{"GET"})
	}
}