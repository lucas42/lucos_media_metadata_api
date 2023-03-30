package main

import (
	"io"
	"log"
	"net/http"
)
/**
* A controller for server 410 Gone responses to endpoints which were part of V1 of the API
*/
func V1GoneController(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusGone)
	io.WriteString(w, "Version 1 of the API is no longer available.  Find version 2 at /v2/tracks\n")
	log.Printf("Request to deprecated v1 endpoint: %s", r.URL.Path)
}