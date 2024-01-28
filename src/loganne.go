package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type LoganneInterface interface {
    post(string, string, Track, Track)
    collectionPost(string, string, Collection, Collection)
}

type Loganne struct {
	source string
	host   string
}

func (loganne Loganne) post(eventType string, humanReadable string, updatedTrack Track, existingTrack Track) {
	url := loganne.host + "/events"
	slog.Debug("Posting to loganne", "eventType", eventType, "humanReadable", humanReadable, "url", url, "updatedTrack", updatedTrack, "existingTrack", existingTrack)

	data := map[string]interface{}{
		"source":  loganne.source,
		"type": eventType,
		"humanReadable": humanReadable,
	}

	// If there's an updated track, include that in the data and link to it in loganne
	if updatedTrack.ID > 0 {
		data["track"] = updatedTrack
		data["url"] = fmt.Sprintf("https://media-metadata.l42.eu/tracks/%d", updatedTrack.ID)

	// If the was an existing track, but no updated one (ie a delete event)
	// include the old track in the data, but don't link to it as the link will 404
	} else if existingTrack.ID > 0 {
		data["track"] = existingTrack
	}
	postData, _ := json.Marshal(data)
	_, err := http.Post(url, "application/json", bytes.NewBuffer(postData))
	if err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
	}
}

func (loganne Loganne) collectionPost(eventType string, humanReadable string, updatedCollection Collection, existingCollection Collection) {
	url := loganne.host + "/events"
	slog.Debug("Posting to loganne", "eventType", eventType, "humanReadable", humanReadable, "url", url, "updatedCollection", updatedCollection, "existingCollection", existingCollection)

	data := map[string]interface{}{
		"source":  loganne.source,
		"type": eventType,
		"humanReadable": humanReadable,
	}

	// If there's an updated collection, include that in the data and link to it in loganne
	if updatedCollection.Slug != "" {
		data["collection"] = updatedCollection
		data["url"] = fmt.Sprintf("https://media-metadata.l42.eu/collections/%s", updatedCollection.Slug)

	// If the was an existing collection, but no updated one (ie a delete event)
	// include the old collection in the data, but don't link to it as the link will 404
	} else if existingCollection.Slug != "" {
		data["collection"] = existingCollection
	}
	postData, _ := json.Marshal(data)
	_, err := http.Post(url, "application/json", bytes.NewBuffer(postData))
	if err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
	}
}
