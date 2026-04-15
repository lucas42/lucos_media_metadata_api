package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

type LoganneInterface interface {
    post(string, string, Track, Track)
    collectionPost(string, string, Collection, Collection)
    albumPost(string, string, AlbumV3, bool)
    albumMergedPost(string, string, AlbumV3, AlbumV3)
}

type Loganne struct {
	source             string
	endpoint           string
	mediaMetadataManagerOrigin string
}

func (loganne Loganne) post(eventType string, humanReadable string, updatedTrack Track, existingTrack Track) {
	slog.Debug("Posting to loganne", "eventType", eventType, "humanReadable", humanReadable, "url", loganne.endpoint, "updatedTrack", updatedTrack, "existingTrack", existingTrack)

	data := map[string]interface{}{
		"source":  loganne.source,
		"type": eventType,
		"humanReadable": humanReadable,
	}

	// If there's an updated track, include that in the data and link to it in loganne
	if updatedTrack.ID > 0 {
		data["track"] = TrackToV3(updatedTrack)
		data["url"] = fmt.Sprintf("%s/tracks/%d", loganne.mediaMetadataManagerOrigin, updatedTrack.ID)

	// If the was an existing track, but no updated one (ie a delete event)
	// include the old track in the data
	} else if existingTrack.ID > 0 {
		data["track"] = TrackToV3(existingTrack)
		data["url"] = fmt.Sprintf("%s/tracks/%d", loganne.mediaMetadataManagerOrigin, existingTrack.ID)
	}
	postData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", loganne.endpoint, bytes.NewBuffer(postData))
	if err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))
	if _, err := http.DefaultClient.Do(req); err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
	}
}

func (loganne Loganne) albumPost(eventType string, humanReadable string, album AlbumV3, withURL bool) {
	slog.Debug("Posting to loganne", "eventType", eventType, "humanReadable", humanReadable, "album", album)

	data := map[string]interface{}{
		"source":        loganne.source,
		"type":          eventType,
		"humanReadable": humanReadable,
		"album":         album,
	}
	if withURL && album.URI != "" {
		data["url"] = album.URI
	}
	postData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", loganne.endpoint, bytes.NewBuffer(postData))
	if err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))
	if _, err := http.DefaultClient.Do(req); err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
	}
}

// albumMergedPost emits an albumMerged Loganne event, matching the entityMerged
// shape: url and targetUri point to the surviving target album; sourceUri identifies
// the album that was merged away. The album field carries the source album data.
func (loganne Loganne) albumMergedPost(eventType string, humanReadable string, sourceAlbum AlbumV3, targetAlbum AlbumV3) {
	slog.Debug("Posting to loganne", "eventType", eventType, "humanReadable", humanReadable, "sourceAlbum", sourceAlbum, "targetAlbum", targetAlbum)

	data := map[string]interface{}{
		"source":        loganne.source,
		"type":          eventType,
		"humanReadable": humanReadable,
		"album":         sourceAlbum,
		"url":           targetAlbum.URI,
		"sourceUri":     sourceAlbum.URI,
		"targetUri":     targetAlbum.URI,
	}
	postData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", loganne.endpoint, bytes.NewBuffer(postData))
	if err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))
	if _, err := http.DefaultClient.Do(req); err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
	}
}

func (loganne Loganne) collectionPost(eventType string, humanReadable string, updatedCollection Collection, existingCollection Collection) {
	slog.Debug("Posting to loganne", "eventType", eventType, "humanReadable", humanReadable, "url", loganne.endpoint, "updatedCollection", updatedCollection, "existingCollection", existingCollection)

	data := map[string]interface{}{
		"source":  loganne.source,
		"type": eventType,
		"humanReadable": humanReadable,
	}

	// If there's an updated collection, include that in the data and link to it in loganne
	if updatedCollection.Slug != "" {
		data["collection"] = updatedCollection
		data["url"] = fmt.Sprintf("%s/collections/%s", loganne.mediaMetadataManagerOrigin, updatedCollection.Slug)

	// If there was an existing collection, but no updated one (ie a delete event)
	// include the old collection in the data, but don't link to it as the link will 404
	} else if existingCollection.Slug != "" {
		data["collection"] = existingCollection
	}
	postData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", loganne.endpoint, bytes.NewBuffer(postData))
	if err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))
	if _, err := http.DefaultClient.Do(req); err != nil {
		slog.Warn("Error occured whilst posting to Loganne", slog.Any("error", err))
	}
}
