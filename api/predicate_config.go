package main

// PredicateConfig holds metadata about a predicate's behaviour.
// Currently governs multi-value storage; designed to be extended
// with RDF mapping, form rendering hints, etc.
type PredicateConfig struct {
	// MultiValue indicates this predicate can have multiple values
	// per track. When true, the database allows multiple tag rows
	// for the same (trackid, predicateid) pair, and the v3 API
	// serialises/deserialises the values as a JSON array.
	MultiValue bool

	// RequiresURI indicates this predicate produces an IRI object in
	// RDF output. Tags with this predicate must have a non-empty uri
	// field to be valid; tags without a URI are skipped by the RDF
	// exporter and rejected by write validation.
	RequiresURI bool

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
}

// predicateRegistry defines per-predicate configuration.
// Predicates not listed here use default behaviour (single-value).
var predicateRegistry = map[string]PredicateConfig{
	"composer": {MultiValue: true},
	"producer": {MultiValue: true},
	"language": {MultiValue: true, RequiresURI: true},
	"offence":  {MultiValue: true},
	"about":    {MultiValue: true, RequiresURI: true},
	"mentions": {MultiValue: true, RequiresURI: true},
	"album": {
		RequiresURI: true,
		ResolveNameToURI: func(store Datastore, name string) (string, error) {
			album, err := store.resolveOrCreateAlbumByName(name)
			return album.URI, err
		},
		ResolveURIToName: func(store Datastore, uri string) (string, error) {
			return store.resolveAlbumNameFromURI(uri)
		},
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
func GetRequiresURIPredicates() []string {
	var predicates []string
	for id, config := range predicateRegistry {
		if config.RequiresURI {
			predicates = append(predicates, id)
		}
	}
	return predicates
}
