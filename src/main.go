package main

import (
	"encoding/json"
	"io"
	"log/slog"
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
	router.HandleFunc("/v2/collections", store.CollectionsV2Controller)
	router.HandleFunc("/v2/collections/", store.CollectionsV2Controller)
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
		slog.Debug("Homepage controller redirect", "path", r.URL.Path)
		http.Redirect(w, r, "/v2/tracks", http.StatusTemporaryRedirect)
	} else {
		slog.Warn("Homepage controller - Not Found", "path", r.URL.Path)
		http.NotFound(w, r)
	}
}

/**
 * Writes a http JSON response based on some data
 */
func writeJSONResponse(w http.ResponseWriter, data interface{}, err error) {
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	if err != nil {
		writeErrorResponse(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

/**
 * Writes a http plain text response based on string (defaults to status code OK)
 */
func writePlainResponse(w http.ResponseWriter, data string, err error) {
	writePlainResponseWithStatus(w, http.StatusOK, data, err)
}
/**
 * Writes a http plain text response based on string and a status code
 */
func writePlainResponseWithStatus(w http.ResponseWriter, status int, data string, err error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	if err != nil {
		writeErrorResponse(w, err)
		return
	}
	w.WriteHeader(status)
	w.Write([]byte(data))
}

/**
 * Writes a http response with no content
 */
func writeContentlessResponse(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
	if err != nil {
		writeErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/**
 * Writes a http response for an error
 */
func writeErrorResponse(w http.ResponseWriter, err error) {
	if strings.HasSuffix(err.Error(), " Not Found") {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else if strings.HasPrefix(err.Error(), "Duplicate:"){
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else if strings.HasSuffix(err.Error(), "not allowed"){
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		slog.Error("Internal Server Error", slog.Any("error", err))
	}
 }

/**
 * Listens for incoming http requests
 * and serve the appropriate response based on the front controller
 *
 * Uses the PORT environment variable to specify which tcp port to listen on (defaults to 8080)
 */
func main() {

	// Check for DEBUG environment variable to drop the log level to Debug
	if os.Getenv("DEBUG") != "" {
		// Can be replaced with `slog.SetLogLoggerLevel(slog.LevelDebug)` in golang 1.22
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

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
	slog.Info("Listening for incoming connections", "port", port)
	err := http.ListenAndServe(":"+port, FrontController(store))
	slog.Error("HTTP server errored", slog.Any("error", err))
	os.Exit(1)
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
