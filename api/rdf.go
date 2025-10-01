package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"github.com/deiu/rdf2go"
)

// RDFHandler serves the RDF file specified by RDF_OUTPUT_PATH.
func RDFHandler(w http.ResponseWriter, r *http.Request) {
	rdfPath := os.Getenv("RDF_OUTPUT_PATH")
	if rdfPath == "" {
		http.Error(w, "RDF_OUTPUT_PATH not set", http.StatusInternalServerError)
		return
	}

	// Pick content type based on extension
	ext := filepath.Ext(rdfPath)
	var contentType string
	switch ext {
	case ".ttl":
		contentType = "text/turtle"
	case ".rdf", ".xml":
		contentType = "application/rdf+xml"
	case ".jsonld":
		contentType = "application/ld+json"
	default:
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	http.ServeFile(w, r, rdfPath)
}

/**
 * Writes a http RDF response based an rdflib Graph
 */
func writeRDFResponse(w http.ResponseWriter, graph *rdf2go.Graph, rdfType string, err error) {
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	if err != nil {
		writeErrorResponse(w, err)
		return
	}
	w.Header().Set("Content-Type", rdfType+"; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	graph.Serialize(w, rdfType)
}

// prefersRDF returns (true, preferred MIME type) if the client prefers RDF,
// false otherwise. Defaults to JSON and "application/json" if nothing matches.
// TODO: Switch to the native http.NegotiateContent once https://go-review.googlesource.com/c/go/+/699455 is released
func prefersRDF(r *http.Request) (bool, string) {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false, "application/json"
	}

	parts := strings.Split(accept, ",")

	rdfTypes := map[string]bool{
		"application/rdf+xml": false, // rdf2go doesn't support xml representation of RDF
		"text/turtle":         true,
		"application/ld+json": true,
	}

	for _, p := range parts {
		mime := strings.TrimSpace(strings.Split(p, ";")[0]) // ignore any ;q=

		if mime == "application/json" {
			return false, "application/json" // JSON takes precedence if listed first
		}

		if rdfTypes[mime] {
			return true, mime
		}
	}

	// default to JSON if nothing matches
	return false, "application/json"
}
