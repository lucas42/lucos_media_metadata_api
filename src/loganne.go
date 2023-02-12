package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type LoganneInterface interface {
    post(string, string, Track, Track)
}

type Loganne struct {
	source string
	host   string
}

func (loganne Loganne) post(eventType string, humanReadable string, updatedTrack Track, existingTrack Track) {
	url := loganne.host + "/events"

	track := updatedTrack
	if track.ID == 0 {
		track = existingTrack
	}
	postData, _ := json.Marshal(map[string]interface{}{
		"source":  loganne.source,
		"type": eventType,
		"humanReadable": humanReadable,
		"track": track,
		"url": fmt.Sprintf("https://media-metadata.l42.eu/tracks/%d", track.ID), // Included for all events, but perhaps less helpful for delete events as the URL will 404?
	})
	_, err := http.Post(url, "application/json", bytes.NewBuffer(postData))
	if err != nil {
		log.Printf("Error occured whilst posting to Loganne %v", err)
	}
}
