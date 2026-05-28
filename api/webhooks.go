package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
)

// LoganneEvent is the shape of an inbound webhook payload from Loganne.
// Only the fields consumed by this service are declared here.
type LoganneEvent struct {
	Type string `json:"type"`
	URL  string `json:"url"`

	// contactLinked (from lucos_contacts) fields
	ContactUri       string  `json:"contactUri"`
	EolasUri         string  `json:"eolasUri"`
	PreviousEolasUri *string `json:"previousEolasUri"` // nil = initial link

	// itemMerged (from lucos_eolas) fields
	SourceUri string `json:"sourceUri"`
	TargetUri string `json:"targetUri"`
}

// entityNameFetcher resolves an entity URI to its current canonical name.
// It is a package-level variable so tests can substitute a mock without
// making real HTTP calls.
var entityNameFetcher func(uri string) (string, error) = fetchEntityNameFromSource

// fetchEntityNameFromSource is the production implementation of entityNameFetcher.
// It routes by hostname:
//   - eolas.l42.eu → fetchEolasName (bulk Turtle endpoint)
func fetchEntityNameFromSource(entityURI string) (string, error) {
	u, err := url.Parse(entityURI)
	if err != nil {
		return "", fmt.Errorf("invalid entity URI %q: %w", entityURI, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("fetchEntityNameFromSource: URI %q has non-HTTPS scheme %q", entityURI, u.Scheme)
	}
	switch u.Hostname() {
	case "eolas.l42.eu":
		return fetchEolasName(entityURI)
	default:
		return "", fmt.Errorf("unrecognised source system host %q in entity URI %q", u.Hostname(), entityURI)
	}
}

// clearTagUrisByUri clears the uri field for all tags whose uri matches entityUri.
// The name (value) field is left unchanged — the tag reverts to a freetext value.
// Returns the number of rows affected.
func (store Datastore) clearTagUrisByUri(entityUri string) (int64, error) {
	result, err := store.DB.Exec(`UPDATE tag SET uri = '' WHERE uri = ?`, entityUri)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

// WebhooksController handles inbound webhook events from external services.
//
//	POST /webhooks
//
// Currently handled event types:
//   - itemDeleted (from lucos_eolas) — clears tag URIs matching the deleted entity's URL
//   - itemUpdated (from lucos_eolas) — refreshes the stored name for matching tag rows
//   - contactLinked (from lucos_contacts) — rewrites tag URIs when a contact is linked to an eolas Person
//   - itemMerged (from lucos_eolas) — rewrites tag URIs when an entity is merged into another
//
// All other event types are acknowledged with 204 and ignored.
func (store Datastore) WebhooksController(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, []string{http.MethodPost})
		return
	}

	var event LoganneEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		slog.Warn("Failed to decode Loganne webhook payload", slog.Any("error", err))
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "itemDeleted":
		if event.URL == "" {
			slog.Warn("Loganne webhook: missing url field", "type", event.Type)
			http.Error(w, "Bad Request: url field is required", http.StatusBadRequest)
			return
		}
		count, err := store.clearTagUrisByUri(event.URL)
		if err != nil {
			slog.Error("Failed to clear tag URIs", slog.Any("error", err), "entityUri", event.URL)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		slog.Info("Cleared orphan tag URIs on entity deletion", "type", event.Type, "entityUri", event.URL, "count", count)

	case "itemUpdated":
		if event.URL == "" {
			// Some eolas itemUpdated events may not carry a url — silently ignore.
			slog.Debug("Loganne webhook: no url field, ignoring", "type", event.Type)
			break
		}
		name, err := entityNameFetcher(event.URL)
		if err != nil {
			// Best-effort: log the failure but do not return a 5xx — Loganne should
			// not retry indefinitely for a transient name-resolution hiccup.
			slog.Warn("Failed to fetch entity name for tag refresh", slog.Any("error", err), "type", event.Type, "entityUri", event.URL)
			break
		}
		count, err := store.updateTagNamesByUri(event.URL, name)
		if err != nil {
			slog.Error("Failed to update tag names", slog.Any("error", err), "entityUri", event.URL)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		slog.Info("Refreshed tag names on entity update", "type", event.Type, "entityUri", event.URL, "updatedCount", count)

	case "contactLinked":
		// Determine the old URI:
		//   - Initial link (previousEolasUri == null): the contact URI was the primary; eolas URI takes over.
		//   - Relink (previousEolasUri != null): the previous eolas URI was the primary; new eolas URI takes over.
		var oldUri string
		if event.PreviousEolasUri != nil && *event.PreviousEolasUri != "" {
			oldUri = *event.PreviousEolasUri
		} else {
			oldUri = event.ContactUri
		}
		newUri := event.EolasUri
		if oldUri == "" || newUri == "" {
			slog.Warn("Loganne webhook: missing uri fields for contactLinked", "type", event.Type, "contactUri", event.ContactUri, "eolasUri", event.EolasUri)
			http.Error(w, "Bad Request: contactUri and eolasUri fields are required", http.StatusBadRequest)
			return
		}
		// URI rewrite is unconditional — the old URI is no longer valid regardless of name-fetch outcome.
		count, err := store.rewriteTagUriOnlyByUri(oldUri, newUri)
		if err != nil {
			slog.Error("Failed to rewrite tag URIs", slog.Any("error", err), "oldUri", oldUri, "newUri", newUri)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		slog.Info("Rewrote tag URIs on contactLinked", "type", event.Type, "oldUri", oldUri, "newUri", newUri, "count", count)
		// Best-effort: refresh the stored name from the new entity. The daily reconciler
		// (reconcileTagNames) will correct any drift if this fetch fails.
		if count > 0 {
			name, err := entityNameFetcher(newUri)
			if err != nil {
				slog.Warn("Failed to fetch entity name after URI rewrite; name will be corrected by daily reconciler", slog.Any("error", err), "type", event.Type, "entityUri", newUri)
				break
			}
			if _, err := store.updateTagNamesByUri(newUri, name); err != nil {
				slog.Warn("Failed to refresh tag name after URI rewrite", slog.Any("error", err), "entityUri", newUri)
			}
		}

	case "itemMerged":
		// Rewrite sourceUri → targetUri and refresh the stored name.
		oldUri := event.SourceUri
		newUri := event.TargetUri
		if oldUri == "" || newUri == "" {
			slog.Warn("Loganne webhook: missing uri fields for entity merge", "type", event.Type, "sourceUri", event.SourceUri, "targetUri", event.TargetUri)
			http.Error(w, "Bad Request: sourceUri and targetUri fields are required", http.StatusBadRequest)
			return
		}
		// URI rewrite is unconditional — the source entity has been merged and the URI is no longer valid.
		count, err := store.rewriteTagUriOnlyByUri(oldUri, newUri)
		if err != nil {
			slog.Error("Failed to rewrite tag URIs", slog.Any("error", err), "oldUri", oldUri, "newUri", newUri)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		slog.Info("Rewrote tag URIs on entity merge", "type", event.Type, "oldUri", oldUri, "newUri", newUri, "count", count)
		// Best-effort: refresh the stored name from the new entity. The daily reconciler
		// (reconcileTagNames) will correct any drift if this fetch fails.
		if count > 0 {
			name, err := entityNameFetcher(newUri)
			if err != nil {
				slog.Warn("Failed to fetch entity name after URI rewrite; name will be corrected by daily reconciler", slog.Any("error", err), "type", event.Type, "entityUri", newUri)
				break
			}
			if _, err := store.updateTagNamesByUri(newUri, name); err != nil {
				slog.Warn("Failed to refresh tag name after URI rewrite", slog.Any("error", err), "entityUri", newUri)
			}
		}

	default:
		slog.Debug("Ignoring unrecognised Loganne event type", "type", event.Type)
	}

	writeContentlessResponse(w, nil)
}
