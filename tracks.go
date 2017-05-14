package main


import (
	"io"
	"net/http"
	"encoding/json"
)

var allTracksByUrl map[string]Track = make(map[string]Track)
func writeAllTrackData(w http.ResponseWriter) {
	io.WriteString(w, "//TODO: all tracks")
}
func writeTrackData(w http.ResponseWriter, trackurl string) {
	track := allTracksByUrl[trackurl]
	if (track.URL != trackurl) {
		http.Error(w, "Track Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(track)
}
type Track struct {
	Fingerprint string `json:"fingerprint"`
	Duration    int `json:"duration"`
	URL         string `json:"url"`
}

func updateTrackData(trackurl string, track Track) (err error) {
	track.URL = trackurl
	allTracksByUrl[trackurl] = track
	return
}
func DecodeTrack(r io.Reader) (Track, error) {
    track := new(Track)
    err := json.NewDecoder(r).Decode(track)
    return *track, err
}
func trackHandling(w http.ResponseWriter, r *http.Request) {
	trackurl := r.URL.Query().Get("url")
	if (len(trackurl) == 0) {
		if (r.Method == "GET") {
			writeAllTrackData(w)
		} else {
			MethodNotAllowed(w, []string{"GET"})
		}
	} else {
		switch r.Method {
			case "PUT":
				track, err := DecodeTrack(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				err = updateTrackData(trackurl, track)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fallthrough
			case "GET":
				writeTrackData(w, trackurl)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT"})
		}
	}
}