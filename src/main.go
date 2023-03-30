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
	router.HandleFunc("/v2/tracks", store.TracksV2Controller)
	router.HandleFunc("/v2/tracks/", store.TracksV2Controller)
	router.HandleFunc("/_info", store.InfoController)
	router.HandleFunc("/", HomepageController)

	/** The following routes used to be part of the V1 API - return a Gone status */
	router.HandleFunc("/tracks", V1GoneController)
	router.HandleFunc("/tracks/", V1GoneController)
	router.HandleFunc("/globals", V1GoneController)
	router.HandleFunc("/globals/", V1GoneController)
	router.HandleFunc("/predicates/", V1GoneController)
	router.HandleFunc("/tags/", V1GoneController)
	router.HandleFunc("/search", V1GoneController)
	return router
}

func HomepageController(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/v2/tracks", http.StatusTemporaryRedirect)
	} else {
		http.NotFound(w, r)
	}
}

/**
 * Writes a http JSON response based on some data
 */
func writeJSONResponse(w http.ResponseWriter, data interface{}, err error) {
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if strings.HasSuffix(err.Error(), " Not Found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Internal Server Error: %s", err.Error())
		}
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

/**
 * Writes a http plain text response based on string
 */
func writePlainResponse(w http.ResponseWriter, data string, err error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	if err != nil {
		if strings.HasSuffix(err.Error(), " Not Found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Internal Server Error: %s", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(data))
}

/**
 * Writes a http response with no content
 */
func writeContentlessResponse(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	if err != nil {
		if strings.HasSuffix(err.Error(), " Not Found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Internal Server Error: %s", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/**
 * Listens for incoming http requests
 * and serve the appropriate response based on the front controller
 *
 * Uses the PORT environment variable to specify which tcp port to listen on (defaults to 8080)
 */
func main() {
	loganne := Loganne{
		host: "https://loganne.l42.eu",
		source: "lucos_media_metadata_api",
	}
	store := DBInit("/var/lib/media-metadata/media.sqlite", loganne)
	var port string
	if len(os.Getenv("PORT")) > 0 {
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
