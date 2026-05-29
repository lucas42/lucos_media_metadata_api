package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"lucos_media_metadata_api/rdfgen"
)

func main() {
	dbPath := os.Getenv("SQLITE_DB_PATH")
	if dbPath == "" {
		log.Fatal("SQLITE_DB_PATH must be set")
	}

	outFile := os.Getenv("RDF_OUTPUT_PATH")
	if outFile == "" {
		log.Fatal("RDF_OUTPUT_PATH must be set")
	}

	scheduleTracker := os.Getenv("SCHEDULE_TRACKER_ENDPOINT")
	if scheduleTracker == "" {
		log.Fatal("SCHEDULE_TRACKER_ENDPOINT must be set")
	}

	if err := rdfgen.ExportRDF(dbPath, outFile); err != nil {
		scheduleTrackerData, _ := json.Marshal(map[string]interface{}{
			"system":    "lucos_media_metadata_api",
			"job_name":  "exporter",
			"frequency": 60*60, // 1 hour in seconds
			"status":    "error",
			"message":   err.Error(),
		})
		postToScheduleTracker(scheduleTracker, scheduleTrackerData)
		log.Fatalf("failed to export RDF: %v", err)
	}

	log.Printf("RDF export written to %s", outFile)
	scheduleTrackerData, _ := json.Marshal(map[string]interface{}{
		"system":    "lucos_media_metadata_api",
		"job_name":  "exporter",
		"frequency": 60*60, // 1 hour in seconds
		"status":    "success",
	})
	postToScheduleTracker(scheduleTracker, scheduleTrackerData)
}

// postToScheduleTracker sends a JSON payload to the schedule tracker endpoint with User-Agent set.
func postToScheduleTracker(endpoint string, data []byte) {
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to create schedule tracker request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))
	if _, err := http.DefaultClient.Do(req); err != nil {
		log.Printf("Failed to post to schedule tracker: %v", err)
	}
}
