package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"lucos_media_metadata_api/rdfgen"
)

func main() {
	dbPath := os.Getenv("SQLITE_DB_PATH")
	if dbPath == "" {
		log.Fatal("SQLITE_DB_PATH must be set")
	}

	outFile := os.Getenv("RDF_OUTPUT_PATH")
	if dbPath == "" {
		log.Fatal("RDF_OUTPUT_PATH must be set")
	}

	scheduleTracker := os.Getenv("SCHEDULE_TRACKER_ENDPOINT")
	if scheduleTracker == "" {
		log.Fatal("SCHEDULE_TRACKER_ENDPOINT must be set")
	}

	tmpDB, cleanup, err := copySQLiteDB(dbPath)
	if err != nil {
		log.Fatalf("failed to copy db: %v", err)
	}
	defer cleanup()

	if err := rdfgen.ExportRDF(tmpDB, outFile); err != nil {
		scheduleTrackerData, _ := json.Marshal(map[string]interface{}{
			"system":    "media_metadata_api_exporter",
			"frequency": 60*60, // 1 hour in seconds
			"status":    "error",
			"message":   err.Error(),
		})
		http.Post(scheduleTracker, "application/json", bytes.NewBuffer(scheduleTrackerData))
		log.Fatalf("failed to export RDF: %v", err)
	}

	log.Printf("RDF export written to %s", outFile)
	scheduleTrackerData, _ := json.Marshal(map[string]interface{}{
		"system":    "media_metadata_api_exporter",
		"frequency": 60*60, // 1 hour in seconds
		"status":    "success",
	})
	http.Post(scheduleTracker, "application/json", bytes.NewBuffer(scheduleTrackerData))
}

// copySQLiteDB creates a temp copy of the SQLite DB file and returns its path
func copySQLiteDB(src string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "sqlite-export")
	if err != nil {
		return "", nil, err
	}
	dst := filepath.Join(tmpDir, "data-export.sqlite")

	srcFile, err := os.Open(src)
	if err != nil {
		return "", nil, err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return "", nil, err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", nil, err
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return dst, cleanup, nil
}