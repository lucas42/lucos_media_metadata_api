package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/jmoiron/sqlx"
)

// reconcileTagNames walks every distinct non-empty URI stored in tags with
// RequiresURI predicates, fetches the current canonical name from eolas,
// and corrects any rows where the stored name has drifted from the canonical
// name.
//
// Routing is by hostname:
//   - eolas.l42.eu → fetched in one bulk call via fetchEolasNames
//   - other hosts   → skipped (logged at debug level)
//
// Reports success or failure to the schedule_tracker endpoint so the daily
// run is monitored.
func (store Datastore) reconcileTagNames() {
	endpoint := os.Getenv("SCHEDULE_TRACKER_ENDPOINT")
	err := store.reconcileTagNamesWithFetchers(fetchEolasNames)
	if err != nil {
		slog.Error("reconcileTagNames: failed", slog.Any("error", err))
		reportToScheduleTracker(endpoint, "error", err.Error())
	} else {
		reportToScheduleTracker(endpoint, "success", "")
	}
}

// reconcileTagNamesWithFetchers is the testable core of reconcileTagNames.
// Production code calls it via reconcileTagNames; tests inject a mock fetcher.
// Returns a non-nil error only for structural failures (DB unavailable, query
// build failure); best-effort per-URI fetch failures are logged but do not
// cause an error return.
func (store Datastore) reconcileTagNamesWithFetchers(
	eolasNamesFetcher func(uris []string) map[string]string,
) error {
	slog.Info("reconcileTagNames: starting")

	// Collect all distinct non-empty URIs that appear in RequiresURI predicate tags.
	predicates := GetRequiresURIPredicates()
	if len(predicates) == 0 {
		slog.Info("reconcileTagNames: no RequiresURI predicates configured, nothing to do")
		return nil
	}

	// sqlx.In expands the slice into individual placeholders for the IN clause.
	query, args, err := sqlx.In(
		`SELECT DISTINCT uri FROM tag WHERE uri != '' AND predicateid IN (?)`,
		predicates,
	)
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}
	query = store.DB.Rebind(query)

	var uris []string
	if err := store.DB.Select(&uris, query, args...); err != nil {
		return fmt.Errorf("failed to query URIs: %w", err)
	}
	if len(uris) == 0 {
		slog.Info("reconcileTagNames: no tagged URIs to reconcile")
		return nil
	}
	slog.Info("reconcileTagNames: found URIs to check", "count", len(uris))

	// Partition by hostname — only eolas URIs are fetched; others are skipped.
	var eolasURIs []string
	for _, uri := range uris {
		u, err := url.Parse(uri)
		if err != nil {
			slog.Warn("reconcileTagNames: skipping unparseable URI", "uri", uri)
			continue
		}
		switch u.Hostname() {
		case "eolas.l42.eu":
			eolasURIs = append(eolasURIs, uri)
		default:
			slog.Debug("reconcileTagNames: skipping URI with unrecognised host", "uri", uri)
		}
	}

	// Fetch names from eolas.
	allNames := make(map[string]string)
	if len(eolasURIs) > 0 {
		names := eolasNamesFetcher(eolasURIs)
		for uri, name := range names {
			allNames[uri] = name
		}
		slog.Info("reconcileTagNames: fetched eolas names", "requested", len(eolasURIs), "resolved", len(names))
	}

	// Update any rows where the name has drifted.
	updatedTotal := int64(0)
	for uri, name := range allNames {
		if name == "" {
			slog.Warn("reconcileTagNames: skipping URI with empty name", "uri", uri)
			continue
		}
		n, err := store.updateTagNamesByUri(uri, name)
		if err != nil {
			slog.Error("reconcileTagNames: failed to update tag name", "uri", uri, slog.Any("error", err))
			continue
		}
		updatedTotal += n
	}

	slog.Info("reconcileTagNames: complete", "total_uri_count", len(uris), "updated_rows", updatedTotal)
	return nil
}

// reportToScheduleTracker posts a status update to the schedule_tracker v2 API.
// If endpoint is empty the call is skipped silently.
func reportToScheduleTracker(endpoint, status, message string) {
	if endpoint == "" {
		slog.Warn("SCHEDULE_TRACKER_ENDPOINT not set, skipping schedule_tracker report")
		return
	}
	payload := map[string]interface{}{
		"system":    "lucos_media_metadata_api",
		"job_name":  "reconcile_tag_names",
		"frequency": 24 * 60 * 60, // daily, in seconds
		"status":    status,
	}
	if message != "" {
		payload["message"] = message
	}
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("reconcileTagNames: failed to marshal schedule_tracker payload", slog.Any("error", err))
		return
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(data))
	if err != nil {
		slog.Warn("reconcileTagNames: failed to build schedule_tracker request", slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", os.Getenv("SYSTEM"))
	if _, err := http.DefaultClient.Do(req); err != nil {
		slog.Warn("reconcileTagNames: failed to post to schedule_tracker", slog.Any("error", err))
	}
}
