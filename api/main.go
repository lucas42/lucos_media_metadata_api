package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
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

	// Dump all goroutine stacks to the log on SIGUSR1 for live deadlock diagnosis.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGUSR1)
		for range ch {
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			slog.Warn("SIGUSR1 received — goroutine dump", "stacks", string(buf[:n]))
		}
	}()

	loganne := Loganne{
		endpoint:           os.Getenv("LOGANNE_ENDPOINT"),
		source:             "lucos_media_metadata_api",
		mediaMetadataManagerOrigin: os.Getenv("MEDIA_METADATA_MANAGER_ORIGIN"),
	}
	store := DBInit("/var/lib/media-metadata/media.sqlite", loganne)
	store.ManagerOrigin = os.Getenv("MEDIA_METADATA_MANAGER_ORIGIN")
	var port string
	if len(os.Getenv("PORT")) > 0 {
		port = os.Getenv("PORT")
	} else {
		port = "8080"
	}
	slog.Info("Listening for incoming connections", "port", port)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           FrontController(store, os.Getenv("CLIENT_KEYS")),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	err := server.ListenAndServe()
	slog.Error("HTTP server errored", slog.Any("error", err))
	os.Exit(1)
}
