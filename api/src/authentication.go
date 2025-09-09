package main

import (
	"log/slog"
	"net/http"
	"strings"
)

/* Middleware which authenticates requests before calling their handler */

type AuthentictedServer struct {
	unauthenticatedHandler http.Handler
	allowedKeys map[string]AuthenticatedClient
}

/**
 * Checks whether the user is authenticated
 * If so, passes on to the relevant handler
 * If not, serves a forbidden page
 */
func (server AuthentictedServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if server.isAuthenticated(request) {
		server.unauthenticatedHandler.ServeHTTP(writer, request)
	} else {
		writer.Header().Set("WWW-Authenticate", "key")
		http.Error(writer, "Authentication Failed", http.StatusUnauthorized)
	}
}

func (server AuthentictedServer) isAuthenticated(request *http.Request) (bool) {
	// Unauthenticated requests to the info path are allowed
	if request.URL.Path == "/_info" {
		return true
	}
	authHeaderParts := strings.Split(request.Header.Get("Authorization"), " ")
	scheme := authHeaderParts[0]
	if scheme != "key" {
		slog.Debug("Unsupported authentication scheme", "scheme", scheme)
		return false
	}
	key := authHeaderParts[1]
	client, found := server.allowedKeys[key]
	if !found {
		slog.Debug("Authentication failed", "key", key)
		return false
	}
	slog.Debug("Request successfully authenticated", "client", client)
	return true
}

type AuthenticatedClient struct {
	System string
	Environment string
}

func parseClientKeys(rawInput string) (map[string]AuthenticatedClient) {
	keys := make(map[string]AuthenticatedClient)
	rawKeys := strings.Split(rawInput, ";")
	for _, rawKey := range rawKeys {
		rawKeyParts := strings.Split(rawKey, "=")
		rawClientInfo := strings.Split(rawKeyParts[0], ":")
		key := strings.TrimSpace(rawKeyParts[1])
		client := AuthenticatedClient{ System: strings.TrimSpace(rawClientInfo[0]), Environment: strings.TrimSpace(rawClientInfo[1]) }
		keys[key] = client
	}
	return keys
}

func NewAuthenticatedServer(unauthenticatedHandler http.Handler, clientKeys string) http.Handler {
	return AuthentictedServer{unauthenticatedHandler, parseClientKeys(clientKeys)}
}