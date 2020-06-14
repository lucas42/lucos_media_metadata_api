package main

import (
	"net/http"
	"strconv"
	"time"
)

type InfoStruct struct {
	System  string            `json:"system"`
	Checks  map[string]Check  `json:"checks"`
	Metrics map[string]Metric `json:"metrics"`
	CI      map[string]string `json:"ci"`
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

		dbCheck := Check{TechDetail: "Does basic SELECT query from database"}
		trackCount := Metric{TechDetail: "Number of tracks in database"}
		err := store.DB.Get(&trackCount.Value, "SELECT COUNT(*) FROM track")
		if err != nil {
			dbCheck.OK = false
			dbCheck.Debug = err.Error()
		} else {
			dbCheck.OK = true
		}

		importRecencyCheck := Check{TechDetail:"Checks whether 'latest_import-timestamp' is within the past fortnight"}
		sinceImport := Metric{TechDetail:"Seconds since latest completion of import script"}
		latestImportTimestamp, err := store.getGlobal("latest_import-timestamp")
		if err != nil {
			importRecencyCheck.OK = false
			importRecencyCheck.Debug = err.Error()
			sinceImport.Value = -1
		} else {
			latestImportTime, err := time.Parse(time.RFC3339, latestImportTimestamp)
			if err != nil {
				importRecencyCheck.OK = false
				importRecencyCheck.Debug = err.Error()
				sinceImport.Value = -1
			} else {
				sinceImport.Value = int(time.Since(latestImportTime).Seconds())
				importRecencyCheck.OK = latestImportTime.After(time.Now().Add(time.Hour * 24 * -14))
			}
		}

		importErrors := Metric{TechDetail:"Number of errors from latest completed run of import script"}
		importErrorsString, _ := store.getGlobal("latest_import-errors")
		importErrorsInt, _ := strconv.Atoi(importErrorsString)
		importErrors.Value = importErrorsInt



		info.Checks = map[string]Check{
			"db": dbCheck,
			"import": importRecencyCheck,
		}
		info.Metrics = map[string]Metric{
			"track-count": trackCount,
			"since-import": sinceImport,
			"import-errors": importErrors,
		}
		info.CI = map[string]string{
			"circle": "gh/lucas42/lucos_media_metadata_api",
		}
		writeJSONResponse(w, info, nil)
	} else {
		MethodNotAllowed(w, []string{"GET"})
	}
}
