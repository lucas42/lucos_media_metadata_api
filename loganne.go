package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
)

type LoganneInterface interface {
    post(string, string, Track)
}

type Loganne struct {
	source string
	host   string
}

func (loganne Loganne) post(eventType string, humanReadable string, track Track) {
	url := loganne.host + "/events"

	postData, _ := json.Marshal(map[string]interface{}{
		"source":  loganne.source,
		"type": eventType,
		"humanReadable": humanReadable,
		"track": track,
	})
	_, err := http.Post(url, "application/json", bytes.NewBuffer(postData))
	if err != nil {
		log.Printf("Error occured whilst posting to Loganne %v", err)
	}
}
