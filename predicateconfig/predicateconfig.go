// Package predicateconfig is the single source of truth for predicate configuration.
// It defines types, interfaces, and utility functions shared by the api and rdfgen
// packages. The concrete per-predicate data lives in registry.go.
package predicateconfig

import (
	"fmt"
	"os"
	"strings"
)

// ValueShape represents the RDF shape of a predicate's value.
type ValueShape int

const (
	// ValueShapeOmit is the zero value — this predicate is not emitted in RDF output.
	ValueShapeOmit ValueShape = iota
	// ValueShapeLiteral — the value column is emitted as an rdf:Literal.
	ValueShapeLiteral
	// ValueShapeURIObject — the tag's uri column (when non-empty) is emitted as an IRI
	// resource. Tags with an empty or nil URI are silently skipped in RDF output.
	// Write validation rejects tags without a URI for these predicates.
	ValueShapeURIObject
	// ValueShapeSearchURL is a transitional shape — synthetic IRIs constructed
	// from ${MEDIA_METADATA_MANAGER_ORIGIN}/search?p.{predicate}={value}. These
	// IRIs aren't real resources (they're search queries dressed as URIs) and
	// can't be merged, sameAs'd, or resolved against any vocabulary. Long-term,
	// each SearchURL predicate should migrate to ValueShape: URIObject with a
	// proper URI scheme. See per-predicate migration tickets in the comment
	// thread of #252.
	//
	// Deprecated: SearchURL is a transitional shape.
	ValueShapeSearchURL
	// ValueShapeMBIDPrefix — the tag's value column is appended to Config.URIPrefix
	// to produce a full IRI resource (e.g. "https://musicbrainz.org/artist/" + value).
	// Used for MusicBrainz ID predicates where the value is a bare UUID and the
	// full URI is mechanically derivable. See per-predicate migration notes in #252
	// for the possible long-term trajectory (D2: materialise the URI into tag.uri
	// at write time and fold into ValueShapeURIObject).
	ValueShapeMBIDPrefix
)

// NameURIResolver resolves names to URIs and vice versa for URI-object predicates
// that support name-based lookup (e.g. album, composer, producer). The api.Datastore
// type implements this interface, allowing the registry closures to call back into the
// database and external services without importing the api package.
type NameURIResolver interface {
	// ResolveOrCreateByName looks up or creates an album-like entity by name.
	// Used exclusively by the album predicate.
	ResolveOrCreateByName(name string) (string, error)
	// ResolveNameFromURI looks up an album name from its URI.
	// Used exclusively by the album predicate.
	ResolveNameFromURI(uri string) (string, error)
	// ResolveOrCreateEolasEntityByName looks up or creates an eolas entity of the
	// given type by name, returning its URI. Used by predicates that store eolas
	// entity references (e.g. composer → person, producer → person).
	ResolveOrCreateEolasEntityByName(entityType, name string) (string, error)
	// ResolveEolasEntityName returns the canonical name for a given eolas entity URI.
	ResolveEolasEntityName(uri string) (string, error)
}

// Symbolic origin identifiers for use in Config.AllowedOrigins.
// ValidateURIOrigin resolves these to actual base URLs at call time via os.Getenv.
const (
	OriginEolas                = "eolas"
	OriginMediaMetadataManager = "media_metadata_manager"
	// OriginMediaMetadataAPI is used for predicates whose concept URIs are served by this service
	// (e.g. SKOS concept schemes at {APP_ORIGIN}/vocab/{predicate}/{slug}).
	OriginMediaMetadataAPI = "media_metadata_api"
)

// Config holds the full configuration for a predicate, covering both its RDF shape
// (used by rdfgen) and its runtime behaviour (used by the api write path, Loganne
// event generation, and URI validation).
//
// PredicateURI is the full RDF predicate IRI. For predicates whose IRI is relative
// to the service's APP_ORIGIN (e.g. custom ontology predicates), the URI starts
// with "/" and must be prefixed with APP_ORIGIN at runtime.
type Config struct {
	// PredicateURI is the full RDF predicate IRI, or a "/" prefix relative to APP_ORIGIN.
	PredicateURI string

	// URIPrefix, when non-empty, is prepended to the tag's value column to produce the
	// full object IRI. Only used by ValueShapeMBIDPrefix predicates.
	// Example: URIPrefix="https://musicbrainz.org/artist/" + value="abc123" → IRI.
	URIPrefix string

	// ValueShape indicates the RDF shape of this predicate's value.
	// ValueShapeURIObject predicates require a non-empty uri field;
	// tags without a URI are rejected by write validation. This drives RequiresURI().
	ValueShape ValueShape

	// MultiValue indicates this predicate can have multiple values per track.
	// When true, the database allows multiple tag rows for the same (trackid,
	// predicateid) pair, and the v3 API serialises/deserialises the values as a
	// JSON array.
	MultiValue bool

	// ResolveNameToURI, if non-nil, enables name-to-URI resolution for this predicate.
	// When a tag value has a name but no URI, the write path calls this to resolve
	// (or create) the entity and populate the URI. Resolution happens before
	// RequiresURI validation.
	ResolveNameToURI func(NameURIResolver, string) (string, error)

	// ResolveURIToName, if non-nil, enables URI-to-name resolution. When a tag value
	// has a URI but no name, the write path calls this to populate the name field
	// before storing.
	ResolveURIToName func(NameURIResolver, string) (string, error)

	// LoganneHumanReadable, if non-nil, returns a bespoke humanReadable message
	// for Loganne events when this predicate is the only non-silent tag changed in
	// an update and no scalar track fields are being modified.
	// Receives the track name as returned by track.getName().
	LoganneHumanReadable func(trackName string) string

	// LoganneSilent marks a predicate as a silent companion that does not affect
	// which Loganne message is emitted. When true, this predicate is ignored when
	// deciding whether to emit a bespoke or generic message.
	LoganneSilent bool

	// AllowedOrigins, if non-nil, holds symbolic origin identifiers (OriginEolas,
	// OriginMediaMetadataManager) whose actual base URLs must prefix URI values of
	// this predicate. Resolved to real URLs at validation time by ValidateURIOrigin.
	// If all identifiers resolve to empty strings (env var unset), validation is skipped.
	AllowedOrigins []string
}

// RequiresURI reports whether this predicate produces an IRI object in RDF output
// and requires a non-empty URI for write validation. Derived from ValueShape.
func (c Config) RequiresURI() bool {
	return c.ValueShape == ValueShapeURIObject
}

// ValidateURIOrigin checks whether the given URI starts with one of the predicate's
// AllowedOrigins. Returns an empty string if the URI is valid (or if no allowlist is
// configured). Returns a human-readable error message if validation fails.
//
// Origin env vars are read at call time so values set after package init (e.g. in
// TestMain or main()) are picked up. Empty values (unset env vars) are excluded;
// if all resolve to empty, validation is skipped entirely.
func (c Config) ValidateURIOrigin(uri string) string {
	if c.AllowedOrigins == nil {
		return ""
	}
	originValues := map[string]string{
		OriginEolas:                os.Getenv("EOLAS_ORIGIN"),
		OriginMediaMetadataManager: os.Getenv("MEDIA_METADATA_MANAGER_ORIGIN"),
		OriginMediaMetadataAPI:     os.Getenv("APP_ORIGIN"),
	}
	validOrigins := make([]string, 0, len(c.AllowedOrigins))
	for _, key := range c.AllowedOrigins {
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
