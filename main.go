package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)


/**
 * The front controller for all incoming requests
 */
func FrontController(store Datastore) *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/tracks", store.TracksController)
	router.HandleFunc("/tracks/", store.TracksController)
	router.HandleFunc("/globals/", store.GlobalsController)
	router.HandleFunc("/predicates/", store.PredicatesController)
	router.HandleFunc("/tags/", store.TagsController)
	redirect := http.RedirectHandler("/tracks", 307)
	router.Handle("/", redirect)
	return router
}

/**
 * Writes a http JSON response based on some data
 */
func writeJSONResponse(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		if strings.HasSuffix(err.Error(), " Not Found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

/**
 * Writes a http plain text response based on string
 */
func writePlainResponse(w http.ResponseWriter, data string, err error) {
	w.Header().Set("Content-Type", "text/plain")
	if err != nil {
		if strings.HasSuffix(err.Error(), " Not Found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(data))
}


/**
 * Listens for incoming http requests
 * and serve the appropriate response based on the front controller
 *
 * Uses the PORT environment variable to specify which tcp port to listen on (defaults to 8080)
 */
func main() {
	store := DBInit("media.sqlite")
	var port string
	if (len(os.Getenv("PORT")) > 0) {
		port = os.Getenv("PORT")
	} else {
		port = "8080"
	}
	log.Printf("Listen on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, FrontController(store)))
}

/**
 * Sends a 405 Method Not Allowed HTTP Error
 * and sets the appropriate Allow header
 */
func MethodNotAllowed(w http.ResponseWriter, allowedMethods []string) {
	concatMethods := strings.Join(allowedMethods, ", ")
	w.Header().Set("Allow", concatMethods)
	w.WriteHeader(http.StatusMethodNotAllowed)
	io.WriteString(w, "Method Not Allowed, must be one of: "+concatMethods)
}