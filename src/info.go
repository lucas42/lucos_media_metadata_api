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
 * Checks the recency of a global value, for use in /_info
 */
func RecencyCheck(store Datastore, scriptName string, maxDaysOld int) (Check, Metric) {

	globalKey := "latest_"+scriptName+"-timestamp"
	recencyCheck := Check{TechDetail:"Checks whether '"+globalKey+"' is within the past "+strconv.Itoa(maxDaysOld)+" days"}
	recencyMetric := Metric{TechDetail:"Seconds since latest completion of "+scriptName+" script"}
	latestRunTimestamp, err := store.getGlobal(globalKey)
	if err != nil {
		recencyCheck.OK = false
		recencyCheck.Debug = err.Error()
		recencyMetric.Value = -1
	} else {

		// Expect format outputted by python datetime's isoformat() function (with no timezone)
		latestRunTime, err := time.Parse("2006-01-02T15:04:05.000000", latestRunTimestamp)
		if err != nil {
			recencyCheck.OK = false
			recencyCheck.Debug = err.Error()
			recencyMetric.Value = -1
		} else {
			recencyMetric.Value = int(time.Since(latestRunTime).Seconds())
			recencyCheck.OK = latestRunTime.After(time.Now().Add(time.Hour * -24 * time.Duration(maxDaysOld)))
		}
	}
	return recencyCheck, recencyMetric
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

		info.Checks = map[string]Check{
			"db": dbCheck,
		}
		info.Metrics = map[string]Metric{
			"track-count": trackCount,
		}
		info.CI = map[string]string{
			"circle": "gh/lucas42/lucos_media_metadata_api",
		}
		writeJSONResponse(w, info, nil)
	} else {
		MethodNotAllowed(w, []string{"GET"})
	}
}
