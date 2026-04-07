package main

import (
	"io"
	"log/slog"
	"net/http"
)
/**
* A controller for server 410 Gone responses to endpoints which were part of V1 of the API
*/
func V1GoneController(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusGone)
	io.WriteString(w, "Version 1 of the API is no longer available.  Find version 3 at /v3/tracks\n")
	slog.Warn("Request to deprecated v1 endpoint", "path", r.URL.Path)
}

/**
* A controller for server 410 Gone responses to endpoints which were part of V2 of the API
*/
func V2GoneController(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusGone)
	io.WriteString(w, "Version 2 of the API is no longer available.  Find version 3 at /v3/tracks\n")
	slog.Warn("Request to deprecated v2 endpoint", "path", r.URL.Path)
}