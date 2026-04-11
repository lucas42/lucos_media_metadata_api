package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jmoiron/sqlx"
)

type InfoStruct struct {
	System  string            `json:"system"`
	Title   string            `json:"title"`
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
		info := InfoStruct{System: "lucos_media_metadata_api", Title: "Media Metadata API"}

		dbCheck, trackCount := TrackCount(store)
		weightingCheck, weightingDrift := WeightingCheck(store)
		collectionsWeightingCheck, collectionsWeightingDrift := CollectionsWeightingCheck(store)
		uriIntegrityCheck, tagsMissingURIs := URIIntegrityCheck(store)

		info.Checks = map[string]Check{
			"db":                   dbCheck,
			"weighting":            weightingCheck,
			"collections-weighting": collectionsWeightingCheck,
			"uri-integrity":        uriIntegrityCheck,
		}
		info.Metrics = map[string]Metric{
			"track-count":                  trackCount,
			"weighting-drift":              weightingDrift,
			"collections-weighting-drift":  collectionsWeightingDrift,
			"tags-missing-uris":            tagsMissingURIs,
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
	weightingDrift = Metric{TechDetail: "Difference between maximum cumulativeweighting and the sum of all weightings"}
	err := store.DB.Get(&weightingDrift.Value, "SELECT CAST(MAX(cum_weighting) - SUM(weighting) AS INT) from track;")
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
func URIIntegrityCheck(store Datastore) (uriCheck Check, missingCount Metric) {
	uriCheck = Check{TechDetail: "Tags with URI-dependent predicates all have a URI set"}
	missingCount = Metric{TechDetail: "Number of tags with a URI-dependent predicate but no URI"}
	predicates := GetRequiresURIPredicates()
	if len(predicates) == 0 {
		uriCheck.OK = true
		return
	}
	query, args, err := sqlx.In(
		"SELECT COUNT(*) FROM tag WHERE predicateid IN (?) AND (uri IS NULL OR uri = '')",
		predicates,
	)
	if err != nil {
		uriCheck.OK = false
		uriCheck.Debug = err.Error()
		return
	}
	query = store.DB.Rebind(query)
	err = store.DB.Get(&missingCount.Value, query, args...)
	if err != nil {
		uriCheck.OK = false
		uriCheck.Debug = err.Error()
	} else if missingCount.Value > 0 {
		uriCheck.OK = false
		uriCheck.Debug = fmt.Sprintf("%d tag(s) with a URI-dependent predicate are missing a URI", missingCount.Value)
	} else {
		uriCheck.OK = true
	}
	return
}

func CollectionsWeightingCheck(store Datastore) (weightingCheck Check, driftingCollectionsCount Metric) {
	weightingCheck = Check{TechDetail: "Whether maximum cumulative weighting for each collection matches the sum of all its weightings"}
	driftingCollectionsCount = Metric{TechDetail: "The number of collections whose maximum cumulative weighting doesn't match the sum of all its weightings"}
	var driftingCollections []string
	err := store.DB.Select(&driftingCollections, "SELECT slug from collection WHERE (SELECT MAX(collection_track.cum_weighting) - SUM (weighting) FROM collection_track LEFT JOIN track on trackid=track.id WHERE collectionslug = collection.slug) > 0.1;") // Comparison to 0.1, rather than zero, to account for float point arithmetic
	driftingCollectionsCount.Value = len(driftingCollections)
	if err != nil {
		weightingCheck.OK = false
		weightingCheck.Debug = err.Error()
	} else if driftingCollectionsCount.Value != 0 {
		weightingCheck.OK = false
		weightingCheck.Debug = "The following collections have a maximum `cum_weighting` value which doesn't match the sum of the `weighting` values for all constituent tracks: "+strings.Join(driftingCollections, ", ")
	} else {
		weightingCheck.OK = true
	}
	return weightingCheck, driftingCollectionsCount
}