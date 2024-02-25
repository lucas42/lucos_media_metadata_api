package main

import (
	"log/slog"
	"net/http"
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
		slog.Debug("Info controller")
		info := InfoStruct{System: "lucos_media_metadata_api"}

		dbCheck, trackCount := TrackCount(store)
		weightingCheck, weightingDrift := WeightingCheck(store)

		info.Checks = map[string]Check{
			"db": dbCheck,
			"weighting": weightingCheck,
		}
		info.Metrics = map[string]Metric{
			"track-count": trackCount,
			"weighting-drift": weightingDrift,
		}
		info.CI = map[string]string{
			"circle": "gh/lucas42/lucos_media_metadata_api",
		}
		writeJSONResponse(w, info, nil)
	} else {
		slog.Warn("Info Controller - MethodNotAllowed", "method", r.Method)
		MethodNotAllowed(w, []string{"GET"})
	}
}

func TrackCount(store Datastore) (Check, Metric) {
	dbCheck := Check{TechDetail: "Does basic SELECT query from database"}
	trackCount := Metric{TechDetail: "Number of tracks in database"}
	err := store.DB.Get(&trackCount.Value, "SELECT COUNT(*) FROM track")
	if err != nil {
		dbCheck.OK = false
		dbCheck.Debug = err.Error()
	} else {
		dbCheck.OK = true
	}
	return dbCheck, trackCount
}
func WeightingCheck(store Datastore) (weightingCheck Check, weightingDrift Metric) {
	weightingCheck = Check{TechDetail: "Does the maximum cumulative weighting value match the sum of all weightings"}
	weightingDrift = Metric{TechDetail: "Difference between maximm cumulativeweighting and the sum of all weightings"}
	err := store.DB.Get(&weightingDrift.Value, "SELECT MAX(cum_weighting) - SUM(weighting) from track;")
	if err != nil {
		weightingCheck.OK = false
		weightingCheck.Debug = err.Error()
	} else if weightingDrift.Value > 0 {
		weightingCheck.OK = false
		weightingCheck.Debug = "The maximum `cum_weighting` value in the `tracks` table is greater than the sum of all the `weighting` values"
	} else if weightingDrift.Value < 0 {
		weightingCheck.OK = false
		weightingCheck.Debug = "The maximum `cum_weighting` value in the `tracks` table is less than the sum of all the `weighting` values"
	} else {
		weightingCheck.OK = true
	}
	return weightingCheck, weightingDrift
}