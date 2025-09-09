package main

import (
	"net/http"
	"os"
	"path/filepath"
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

	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, rdfPath)
}
