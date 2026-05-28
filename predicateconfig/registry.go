package predicateconfig

// registry holds the configuration for all known predicates.
// Predicates not listed here use zero-value Config (single-value, Omit shape, default behaviour).
var registry = map[string]Config{
	// MBIDPrefix predicates — MusicBrainz IDs; full IRI = URIPrefix + value.
	"mbid_artist": {
		ValueShape:   ValueShapeMBIDPrefix,
		PredicateURI: "http://purl.org/dc/terms/creator",
		URIPrefix:    "https://musicbrainz.org/artist/",
	},
	"mbid_recording": {
		ValueShape:   ValueShapeMBIDPrefix,
		PredicateURI: "http://purl.org/dc/terms/identifier",
		URIPrefix:    "https://musicbrainz.org/recording/",
	},
	"mbid_release": {
		ValueShape:   ValueShapeMBIDPrefix,
		PredicateURI: "http://purl.org/dc/terms/isPartOf",
		URIPrefix:    "https://musicbrainz.org/release/",
	},

	// SearchURL predicates — transitional; see ValueShapeSearchURL for context.
	// TODO(#237): composer and producer are the last remaining SearchURL predicates.
	// Once #237 (migrate Person-typed tags to eolas URIs) is done, ValueShapeSearchURL
	// and getSearchUrl can be removed entirely.
	"artist": {
		ValueShape:   ValueShapeSearchURL,
		PredicateURI: "http://xmlns.com/foaf/0.1/maker",
	},
	"composer": {
		MultiValue:   true,
		ValueShape:   ValueShapeSearchURL,
		PredicateURI: "http://purl.org/ontology/mo/composer",
	},
	"producer": {
		MultiValue:   true,
		ValueShape:   ValueShapeSearchURL,
		PredicateURI: "http://purl.org/ontology/mo/producer",
	},
	// genre: no consumers, no vocabulary; dormant data left in place.
	// When a genuine use case arrives, file a fresh design ticket.
	// Same pattern as fingerprint_version's Omit registration.
	"genre": {ValueShape: ValueShapeOmit},
	"language": {
		MultiValue:     true,
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#trackLanguage",
		AllowedOrigins: []string{OriginEolas},
	},
	"offence": {
		MultiValue:     true,
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#trigger",
		AllowedOrigins: []string{OriginEolas},
	},
	"about": {
		MultiValue:     true,
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#about",
		AllowedOrigins: []string{OriginEolas},
	},
	"mentions": {
		MultiValue:     true,
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#mentions",
		AllowedOrigins: []string{OriginEolas},
	},
	"theme_tune": {
		MultiValue:     true,
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#theme_tune",
		AllowedOrigins: []string{OriginEolas},
	},
	"soundtrack": {
		MultiValue:     true,
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#soundtrack",
		AllowedOrigins: []string{OriginEolas},
	},
	"provenance": {
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "http://purl.org/dc/terms/source",
		AllowedOrigins: []string{OriginMediaMetadataAPI},
		ResolveNameToURI: SKOSResolveNameToURI("provenance"),
		ResolveURIToName: SKOSResolveURIToName("provenance"),
	},
	"availability": {
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#availability",
		AllowedOrigins: []string{OriginMediaMetadataAPI},
		ResolveNameToURI: SKOSResolveNameToURI("availability"),
		ResolveURIToName: SKOSResolveURIToName("availability"),
	},

	"album": {
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#onAlbum",
		AllowedOrigins: []string{OriginMediaMetadataManager},
		ResolveNameToURI: func(r NameURIResolver, name string) (string, error) {
			return r.ResolveOrCreateByName(name)
		},
		ResolveURIToName: func(r NameURIResolver, uri string) (string, error) {
			return r.ResolveNameFromURI(uri)
		},
	},

	// Literal predicates — value column → rdf:Literal.
	// Well-known predicates use external vocabulary URIs where appropriate.
	"added": {
		ValueShape:   ValueShapeLiteral,
		PredicateURI: "/ontology#dateAdded",
	},
	"title": {
		ValueShape:   ValueShapeLiteral,
		PredicateURI: "http://www.w3.org/2004/02/skos/core#prefLabel",
	},
	"comment": {
		ValueShape:   ValueShapeLiteral,
		PredicateURI: "http://schema.org/comment",
	},
	"lyrics": {
		ValueShape:   ValueShapeLiteral,
		PredicateURI: "http://purl.org/ontology/mo/lyrics",
	},
	"rating": {
		ValueShape:   ValueShapeLiteral,
		PredicateURI: "http://schema.org/ratingValue",
	},
	"memory": {
		ValueShape:   ValueShapeLiteral,
		PredicateURI: "/ontology#memory",
	},
	"year": {
		ValueShape:   ValueShapeLiteral,
		PredicateURI: "http://purl.org/dc/terms/date",
	},

	// Omit predicates — behavioural only, not emitted in RDF output.
	"lastSuccessfulPlay": {
		ValueShape:           ValueShapeOmit,
		LoganneHumanReadable: func(trackName string) string { return "Track " + trackName + " finished playing" },
	},
	"lastError": {
		ValueShape:           ValueShapeOmit,
		LoganneHumanReadable: func(trackName string) string { return "Track " + trackName + " errored" },
	},
	"lastSkip": {
		ValueShape:           ValueShapeOmit,
		LoganneHumanReadable: func(trackName string) string { return "Track " + trackName + " skipped" },
	},
	// lastErrorMessage is a silent companion to lastError: it stores the error
	// string from the client but does not affect Loganne message selection, so
	// a PATCH carrying both tags still emits the bespoke "Track X errored" message.
	"lastErrorMessage": {
		ValueShape:    ValueShapeOmit,
		LoganneSilent: true,
	},
	// fingerprint_version: pending DROP decision (see #264). Dormant data.
	"fingerprint_version": {ValueShape: ValueShapeOmit},

	"singalong": {
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#singalong",
		AllowedOrigins: []string{OriginMediaMetadataAPI},
		ResolveNameToURI: SKOSResolveNameToURI("singalong"),
		ResolveURIToName: SKOSResolveURIToName("singalong"),
	},
	"dance": {
		ValueShape:     ValueShapeURIObject,
		PredicateURI:   "/ontology#dance",
		AllowedOrigins: []string{OriginMediaMetadataAPI},
		ResolveNameToURI: SKOSResolveNameToURI("dance"),
		ResolveURIToName: SKOSResolveURIToName("dance"),
	},
}

// Get returns the Config for predicateID and whether it was found.
// Predicates not in the registry are omitted from RDF output — mapPredicate returns ("", nil) for unknown predicates.
func Get(predicateID string) (Config, bool) {
	c, ok := registry[predicateID]
	return c, ok
}

// GetConfig returns the Config for predicateID.
// If not in the registry, returns a zero-value Config (single-value, Omit shape, default behaviour).
func GetConfig(predicateID string) Config {
	return registry[predicateID]
}

// IsMultiValue reports whether predicateID allows multiple values per track.
func IsMultiValue(predicateID string) bool {
	return registry[predicateID].MultiValue
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

// All returns a copy of the full registry map.
// Used by database migration code that needs to iterate all predicates.
func All() map[string]Config {
	out := make(map[string]Config, len(registry))
	for k, v := range registry {
		out[k] = v
	}
	return out
}
