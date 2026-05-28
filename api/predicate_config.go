package main

import (
	"fmt"
	"strings"

	"lucos_media_metadata_api/predicateconfig"
)

// PredicateConfig holds metadata about a predicate's behaviour.
// RDF-shape fields (PredicateURI, ValueShape) mirror the canonical values in the
// predicateconfig package, which is the single source of truth used by rdfgen.
// Api-specific fields (MultiValue, Resolve*, Loganne*, AllowedOrigins) live here only.
type PredicateConfig struct {
	// MultiValue indicates this predicate can have multiple values
	// per track. When true, the database allows multiple tag rows
	// for the same (trackid, predicateid) pair, and the v3 API
	// serialises/deserialises the values as a JSON array.
	MultiValue bool

	// ValueShape indicates the RDF shape of this predicate's value.
	// ValueShapeURIObject predicates require a non-empty uri field;
	// tags without a URI are rejected by write validation. This mirrors
	// the canonical value in predicateconfig and drives RequiresURI().
	ValueShape predicateconfig.ValueShape

	// PredicateURI is the full RDF predicate IRI (or a "/" prefix relative
	// to APP_ORIGIN). Mirrors the canonical value in predicateconfig; present
	// here for documentation and potential future use by api-layer code.
	PredicateURI string

	// ResolveNameToURI, if non-nil, enables name-to-URI resolution for this
	// predicate. When a tag value has a name but no URI, the write path calls
	// this to resolve (or create) the entity and populate the URI. Resolution
	// happens before RequiresURI validation. In the IfMissing path, resolution
	// only fires for tags that will actually be written.
	ResolveNameToURI func(store Datastore, name string) (string, error)

	// ResolveURIToName, if non-nil, enables URI-to-name resolution. When a tag
	// value has a URI but no name, the write path calls this to populate the
	// name field before storing.
	ResolveURIToName func(store Datastore, uri string) (string, error)

	// LoganneHumanReadable, if non-nil, returns a bespoke humanReadable message
	// for Loganne events when this predicate is the only tag changed in an update
	// and no scalar track fields (fingerprint, URL, duration) are being modified.
	// Receives the track name as returned by track.getName() (e.g. `"Tuesday's Gone"` or `#42`).
	LoganneHumanReadable func(trackName string) string

	// LoganneSilent marks a predicate as a silent companion that does not affect
	// which Loganne message is emitted. When true, this predicate is ignored when
	// getBespokeLoganneMessage decides whether to emit a bespoke or generic message.
	// This allows a primary predicate (e.g. lastError) to carry a bespoke message
	// even when paired with a companion predicate (e.g. lastErrorMessage) in the
	// same PATCH request.
	LoganneSilent bool

	// AllowedOrigins, if non-nil, holds the set of symbolic origin identifiers
	// (see originEolas, originMediaMetadataManager) whose actual base URLs must
	// prefix URI values of this predicate. Resolved to real URLs at validation
	// time by validateURIOrigin. If all identifiers resolve to empty strings
	// (env var unset), validation is skipped.
	AllowedOrigins []string
}

// RequiresURI reports whether this predicate produces an IRI object in RDF output and
// requires a non-empty uri field for write validation. Derived from ValueShape.
func (config PredicateConfig) RequiresURI() bool {
	return config.ValueShape == predicateconfig.ValueShapeURIObject
}

// Symbolic origin identifiers for use in PredicateConfig.AllowedOrigins.
// validateURIOrigin resolves these to actual base URLs at call time.
const (
	originEolas                = "eolas"
	originMediaMetadataManager = "media_metadata_manager"
)

// validateURIOrigin checks whether the given URI starts with one of the
// predicate's AllowedOrigins. Returns an empty string if the URI is valid (or
// if no allowlist is configured). Returns a human-readable error message if
// validation fails.
func (config PredicateConfig) validateURIOrigin(uri string) string {
	if config.AllowedOrigins == nil {
		return ""
	}
	// Resolve symbolic origin identifiers to actual base URLs. Read at call
	// time so env-var values set after package init (main(), TestMain) are
	// picked up. Empty values (unset env var) are excluded; if all resolve to
	// empty, validation is skipped.
	originValues := map[string]string{
		originEolas:                eolasOrigin,
		originMediaMetadataManager: mediaMetadataManagerOrigin,
	}
	validOrigins := make([]string, 0, len(config.AllowedOrigins))
	for _, key := range config.AllowedOrigins {
		if val := originValues[key]; val != "" {
			validOrigins = append(validOrigins, val)
		}
	}
	if len(validOrigins) == 0 {
		return ""
	}
	for _, origin := range validOrigins {
		if uri == origin || strings.HasPrefix(uri, origin+"/") {
			return ""
		}
	}
	return fmt.Sprintf("uri %q does not start with an allowed origin %v", uri, validOrigins)
}

// predicateRegistry defines per-predicate configuration.
// Predicates not listed here use default behaviour (single-value, no URI required).
var predicateRegistry = map[string]PredicateConfig{
	"composer": {
		MultiValue:   true,
		PredicateURI: "http://purl.org/ontology/mo/composer",
	},
	"producer": {
		MultiValue:   true,
		PredicateURI: "http://purl.org/ontology/mo/producer",
	},
	"language": {
		MultiValue:     true,
		ValueShape:     predicateconfig.ValueShapeURIObject,
		PredicateURI:   "/ontology#trackLanguage",
		AllowedOrigins: []string{originEolas},
	},
	"offence": {
		MultiValue:   true,
		PredicateURI: "/ontology#trigger",
	},
	"about": {
		MultiValue:     true,
		ValueShape:     predicateconfig.ValueShapeURIObject,
		PredicateURI:   "/ontology#about",
		AllowedOrigins: []string{originEolas},
	},
	"mentions": {
		MultiValue:     true,
		ValueShape:     predicateconfig.ValueShapeURIObject,
		PredicateURI:   "/ontology#mentions",
		AllowedOrigins: []string{originEolas},
	},
	"theme_tune": {
		MultiValue:     true,
		ValueShape:     predicateconfig.ValueShapeURIObject,
		PredicateURI:   "/ontology#theme_tune",
		AllowedOrigins: []string{originEolas},
	},
	"soundtrack": {
		MultiValue:     true,
		ValueShape:     predicateconfig.ValueShapeURIObject,
		PredicateURI:   "/ontology#soundtrack",
		AllowedOrigins: []string{originEolas},
	},
	"album": {
		ValueShape:     predicateconfig.ValueShapeURIObject,
		PredicateURI:   "/ontology#onAlbum",
		AllowedOrigins: []string{originMediaMetadataManager},
		ResolveNameToURI: func(store Datastore, name string) (string, error) {
			album, err := store.resolveOrCreateAlbumByName(name)
			return album.URI, err
		},
		ResolveURIToName: func(store Datastore, uri string) (string, error) {
			return store.resolveAlbumNameFromURI(uri)
		},
	},
	// Literal predicates — value column → rdf:Literal.
	"added": {
		ValueShape:   predicateconfig.ValueShapeLiteral,
		PredicateURI: "/ontology#dateAdded",
	},
	"title": {
		ValueShape:   predicateconfig.ValueShapeLiteral,
		PredicateURI: "http://www.w3.org/2004/02/skos/core#prefLabel",
	},
	"comment": {
		ValueShape:   predicateconfig.ValueShapeLiteral,
		PredicateURI: "http://schema.org/comment",
	},
	"lyrics": {
		ValueShape:   predicateconfig.ValueShapeLiteral,
		PredicateURI: "http://purl.org/ontology/mo/lyrics",
	},
	"rating": {
		ValueShape:   predicateconfig.ValueShapeLiteral,
		PredicateURI: "http://schema.org/ratingValue",
	},
	"memory": {
		ValueShape:   predicateconfig.ValueShapeLiteral,
		PredicateURI: "/ontology#memory",
	},
	"year": {
		ValueShape:   predicateconfig.ValueShapeLiteral,
		PredicateURI: "http://purl.org/dc/terms/date",
	},
	// Omit predicates — behavioural only, not emitted in RDF output.
	"lastSuccessfulPlay": {
		ValueShape:           predicateconfig.ValueShapeOmit,
		LoganneHumanReadable: func(trackName string) string { return "Track " + trackName + " finished playing" },
	},
	"lastError": {
		ValueShape:           predicateconfig.ValueShapeOmit,
		LoganneHumanReadable: func(trackName string) string { return "Track " + trackName + " errored" },
	},
	"lastSkip": {
		ValueShape:           predicateconfig.ValueShapeOmit,
		LoganneHumanReadable: func(trackName string) string { return "Track " + trackName + " skipped" },
	},
	// lastErrorMessage is a silent companion to lastError: it stores the error
	// string from the client but does not affect Loganne message selection, so
	// a PATCH carrying both tags still emits the bespoke "Track X errored" message.
	"lastErrorMessage": {
		ValueShape:    predicateconfig.ValueShapeOmit,
		LoganneSilent: true,
	},
}

// GetPredicateConfig returns the configuration for a predicate.
// Predicates not in the registry return a zero-value config
// (single-value, default behaviour).
func GetPredicateConfig(predicateID string) PredicateConfig {
	if config, ok := predicateRegistry[predicateID]; ok {
		return config
	}
	return PredicateConfig{}
}

// IsMultiValue is a convenience for the most common check.
func IsMultiValue(predicateID string) bool {
	return GetPredicateConfig(predicateID).MultiValue
}

// GetRequiresURIPredicates returns the list of predicate IDs that require a URI.
// Used by the RDF exporter, write validation, and audit checks.
// Delegates to predicateconfig as the single source of truth for RDF shapes.
func GetRequiresURIPredicates() []string {
	return predicateconfig.URIObjectPredicates()
}
