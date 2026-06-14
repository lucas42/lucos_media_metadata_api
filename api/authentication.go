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
 * Checks whether the user is authenticated and authorised.
 * If so, passes on to the relevant handler.
 * If the key is unknown, serves a 401 Unauthorized response.
 * If the key is valid but the scope doesn't allow this request, serves a 403 Forbidden response.
 */
func (server AuthentictedServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	authenticated, authorized := server.checkAuth(request)
	if !authenticated {
		writer.Header().Set("WWW-Authenticate", "bearer")
		http.Error(writer, "Authentication Failed", http.StatusUnauthorized)
		return
	}
	if !authorized {
		http.Error(writer, "Insufficient Scope", http.StatusForbidden)
		return
	}
	server.unauthenticatedHandler.ServeHTTP(writer, request)
}

// checkAuth returns (authenticated, authorized).
// authenticated=false means no valid key found (should yield 401).
// authenticated=true, authorized=false means valid key but the scope doesn't cover this request (should yield 403).
func (server AuthentictedServer) checkAuth(request *http.Request) (bool, bool) {
	// Unauthenticated requests to the info and ontology paths are always allowed
	if request.URL.Path == "/_info" || request.URL.Path == "/ontology" {
		return true, true
	}
	authHeaderParts := strings.Split(request.Header.Get("Authorization"), " ")
	scheme := strings.ToLower(authHeaderParts[0])
	if scheme != "bearer" {
		slog.Debug("Unsupported authentication scheme", "scheme", scheme)
		return false, false
	}
	if len(authHeaderParts) < 2 {
		slog.Debug("Missing token in Authorization header", "scheme", scheme)
		return false, false
	}
	key := authHeaderParts[1]
	client, found := server.allowedKeys[key]
	if !found {
		slog.Debug("Authentication failed", "key", key)
		return false, false
	}
	slog.Debug("Request successfully authenticated", "client", client)
	return true, client.isAuthorized(request)
}

// isAuthorized checks whether the client's scopes permit the given request.
//
// Current estate-vocabulary scope strings:
//   - "media-metadata:read"  — permits any GET request (covers /v3/* and /v2/export)
//   - "media-metadata:write" — permits POST/PATCH/DELETE on /v3/* paths
//   - "webhook"              — permits POST /webhooks only (estate-wide bundled scope)
//
// Legacy scope strings (dual-accept during migration; remove after production creds migrated):
//   - "full"        — permits any path and method (use media-metadata:read + media-metadata:write instead)
//   - "export:read" — permits GET /v2/export only (use media-metadata:read instead)
//
// A client with no scopes is fail-closed: all paths return false.
func (client AuthenticatedClient) isAuthorized(request *http.Request) bool {
	for _, scope := range client.Scopes {
		switch scope {
		case "media-metadata:read":
			if request.Method == http.MethodGet {
				return true
			}
		case "media-metadata:write":
			if (request.Method == http.MethodPost || request.Method == http.MethodPatch || request.Method == http.MethodDelete) &&
				strings.HasPrefix(request.URL.Path, "/v3/") {
				return true
			}
		case "webhook":
			if request.Method == http.MethodPost && request.URL.Path == "/webhooks" {
				return true
			}
		// Legacy scopes — dual-accept during migration
		case "full":
			return true
		case "export:read":
			if request.Method == http.MethodGet && request.URL.Path == "/v2/export" {
				return true
			}
		}
	}
	return false
}

type AuthenticatedClient struct {
	System string
	Environment string
	Scopes []string
}

func parseClientKeys(rawInput string) (map[string]AuthenticatedClient) {
	keys := make(map[string]AuthenticatedClient)
	rawKeys := strings.Split(rawInput, ";")
	for _, rawKey := range rawKeys {
		rawKeyParts := strings.Split(rawKey, "=")
		rawClientInfo := strings.Split(rawKeyParts[0], ":")
		// Support optional scope annotation: key=value|scope1,scope2
		keyAndScope := strings.SplitN(rawKeyParts[1], "|", 2)
		key := strings.TrimSpace(keyAndScope[0])
		var scopes []string
		if len(keyAndScope) > 1 {
			for _, s := range strings.Split(keyAndScope[1], ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					scopes = append(scopes, s)
				}
			}
		}
		client := AuthenticatedClient{
			System: strings.TrimSpace(rawClientInfo[0]),
			Environment: strings.TrimSpace(rawClientInfo[1]),
			Scopes: scopes,
		}
		keys[key] = client
	}
	return keys
}

func NewAuthenticatedServer(unauthenticatedHandler http.Handler, clientKeys string) http.Handler {
	return AuthentictedServer{unauthenticatedHandler, parseClientKeys(clientKeys)}
}
