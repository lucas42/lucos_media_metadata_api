package main

import (
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
	redirect := http.RedirectHandler("/tracks", 307)
	router.Handle("/", redirect)
	return router
}

/**
 * Listens for incoming http requests
 * and serve the appropriate response based on the front controller
 *
 * Uses the PORT environment variable to specify which tcp port to listen on (defaults to 8080)
 */
func main() {
	store, err := DBInit("media.sqlite")
	if (err != nil) {
		log.Fatal(err)
		return
	}
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