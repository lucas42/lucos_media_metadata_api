package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// LoganneEvent is the shape of an inbound webhook payload from Loganne.
// Only the fields consumed by this service are declared here.
type LoganneEvent struct {
	Type string `json:"type"`
	URL  string `json:"url"`
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

// LoganneWebhookController handles inbound Loganne webhook events.
//
//	POST /loganne
//
// Currently handled event types:
//   - itemDeleted   (from lucos_eolas) — clears tag URIs matching the deleted entity's URL
//   - contactDeleted (from lucos_contacts) — same behaviour
//
// All other event types are acknowledged with 204 and ignored.
func (store Datastore) LoganneWebhookController(w http.ResponseWriter, r *http.Request) {
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
	case "itemDeleted", "contactDeleted":
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
	default:
		slog.Debug("Ignoring unrecognised Loganne event type", "type", event.Type)
	}

	writeContentlessResponse(w, nil)
}
