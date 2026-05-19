package main

import (
	"log/slog"
	"net/url"

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
func (store Datastore) reconcileTagNames() {
	store.reconcileTagNamesWithFetchers(fetchEolasNames)
}

// reconcileTagNamesWithFetchers is the testable core of reconcileTagNames.
// Production code calls it via reconcileTagNames; tests inject a mock fetcher.
func (store Datastore) reconcileTagNamesWithFetchers(
	eolasNamesFetcher func(uris []string) map[string]string,
) {
	slog.Info("reconcileTagNames: starting")

	// Collect all distinct non-empty URIs that appear in RequiresURI predicate tags.
	predicates := GetRequiresURIPredicates()
	if len(predicates) == 0 {
		slog.Info("reconcileTagNames: no RequiresURI predicates configured, nothing to do")
		return
	}

	// sqlx.In expands the slice into individual placeholders for the IN clause.
	query, args, err := sqlx.In(
		`SELECT DISTINCT uri FROM tag WHERE uri != '' AND predicateid IN (?)`,
		predicates,
	)
	if err != nil {
		slog.Error("reconcileTagNames: failed to build query", slog.Any("error", err))
		return
	}
	query = store.DB.Rebind(query)

	var uris []string
	if err := store.DB.Select(&uris, query, args...); err != nil {
		slog.Error("reconcileTagNames: failed to query URIs", slog.Any("error", err))
		return
	}
	if len(uris) == 0 {
		slog.Info("reconcileTagNames: no tagged URIs to reconcile")
		return
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
}
