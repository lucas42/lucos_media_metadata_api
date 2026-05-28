// Package predicateconfig is the single source of truth for predicate RDF shapes.
// It is imported by both api/ and rdfgen/ so that a single registry drives both
// write-validation logic (RequiresURI) and RDF generation (PredicateURI + ValueShape).
package predicateconfig

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
)

// Config holds the RDF-specific configuration for a predicate.
//
// PredicateURI is the full RDF predicate IRI. For predicates whose IRI is relative
// to the service's APP_ORIGIN (e.g. custom ontology predicates), the URI starts
// with "/" and must be prefixed with APP_ORIGIN at runtime.
type Config struct {
	PredicateURI string
	ValueShape   ValueShape
}

// registry holds RDF configuration for all predicates handled by PredicateConfig lookup.
// Predicates absent from this map fall through to the remaining switch cases in rdfgen.
var registry = map[string]Config{
	// Literal predicates — value column → rdf:Literal.
	"added":   {PredicateURI: "/ontology#dateAdded", ValueShape: ValueShapeLiteral},
	"title":   {PredicateURI: "http://www.w3.org/2004/02/skos/core#prefLabel", ValueShape: ValueShapeLiteral},
	"comment": {PredicateURI: "http://schema.org/comment", ValueShape: ValueShapeLiteral},
	"lyrics":  {PredicateURI: "http://purl.org/ontology/mo/lyrics", ValueShape: ValueShapeLiteral},
	"rating":  {PredicateURI: "http://schema.org/ratingValue", ValueShape: ValueShapeLiteral},
	"memory":  {PredicateURI: "/ontology#memory", ValueShape: ValueShapeLiteral},
	"year":    {PredicateURI: "http://purl.org/dc/terms/date", ValueShape: ValueShapeLiteral},

	// URIObject predicates — tag.uri (when non-empty) → rdf:Resource; tags without URI are skipped.
	"album":      {PredicateURI: "/ontology#onAlbum", ValueShape: ValueShapeURIObject},
	"language":   {PredicateURI: "/ontology#trackLanguage", ValueShape: ValueShapeURIObject},
	"about":      {PredicateURI: "/ontology#about", ValueShape: ValueShapeURIObject},
	"mentions":   {PredicateURI: "/ontology#mentions", ValueShape: ValueShapeURIObject},
	"soundtrack": {PredicateURI: "/ontology#soundtrack", ValueShape: ValueShapeURIObject},
	"theme_tune": {PredicateURI: "/ontology#theme_tune", ValueShape: ValueShapeURIObject},

	// Omit predicates — behavioural only, not emitted in RDF output.
	"lastSuccessfulPlay": {ValueShape: ValueShapeOmit},
	"lastError":          {ValueShape: ValueShapeOmit},
	"lastSkip":           {ValueShape: ValueShapeOmit},
	"lastErrorMessage":   {ValueShape: ValueShapeOmit},
}

// Get returns the Config for predicateID and whether it was found.
// Predicates not in the registry fall through to any remaining switch logic in rdfgen.
func Get(predicateID string) (Config, bool) {
	c, ok := registry[predicateID]
	return c, ok
}

// RequiresURI reports whether predicateID is a URIObject predicate that requires a
// non-empty URI for write validation and RDF emission.
func RequiresURI(predicateID string) bool {
	c, ok := registry[predicateID]
	return ok && c.ValueShape == ValueShapeURIObject
}

// URIObjectPredicates returns all predicate IDs with ValueShape == ValueShapeURIObject.
// Used by write-validation and integrity-check logic.
func URIObjectPredicates() []string {
	predicates := make([]string, 0)
	for id, c := range registry {
		if c.ValueShape == ValueShapeURIObject {
			predicates = append(predicates, id)
		}
	}
	return predicates
}
