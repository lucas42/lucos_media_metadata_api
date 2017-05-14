package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"encoding/json"
)

func fingerprint(w http.ResponseWriter, r *http.Request) {
	id := strings.Replace(r.URL.Path, "/fingerprint/", "", 1)
	switch r.Method {
		case "GET":
			io.WriteString(w, "Hello fingerprint! "+id)
		case "PUT":
			io.WriteString(w, "PUT not done yet "+id)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT"})
	}
}
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
func MethodNotAllowed(w http.ResponseWriter, allowedMethods []string) {
	concatMethods := strings.Join(allowedMethods, ", ")
	w.Header().Set("Allow", concatMethods)
	w.WriteHeader(http.StatusMethodNotAllowed)
	io.WriteString(w, "Method Not Allowed, must be one of: "+concatMethods)
}
func routing() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/fingerprint/", fingerprint)
	router.HandleFunc("/tracks", trackHandling)
	redirect := http.RedirectHandler("/fingerprint/", 307)
	router.Handle("/", redirect)
	return router
}

func main() {
	var port string
	if (len(os.Getenv("PORT")) > 0) {
		port = os.Getenv("PORT")
	} else {
		port = "8080"
	}
	router := routing()
	log.Printf("Listen on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
