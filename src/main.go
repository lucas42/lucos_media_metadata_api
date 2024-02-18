package main

import (
	"log/slog"
	"net/http"
	"os"
)

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
	err := http.ListenAndServe(":"+port, FrontController(store, os.Getenv("CLIENT_KEYS")))
	slog.Error("HTTP server errored", slog.Any("error", err))
	os.Exit(1)
}
