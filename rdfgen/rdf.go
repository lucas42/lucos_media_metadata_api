package rdfgen

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/deiu/rdf2go"

	"lucos_media_metadata_api/predicateconfig"
)

// SKOS and XSD URIs used in the SKOS concept scheme declarations.
const (
	skosConceptScheme = "http://www.w3.org/2004/02/skos/core#ConceptScheme"
	skosConcept       = "http://www.w3.org/2004/02/skos/core#Concept"
	skosInScheme      = "http://www.w3.org/2004/02/skos/core#inScheme"
	skosNotation      = "http://www.w3.org/2004/02/skos/core#notation"
	skosPrefLabel     = "http://www.w3.org/2004/02/skos/core#prefLabel"
	rdfType           = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	rdfsComment       = "http://www.w3.org/2000/01/rdf-schema#comment"
	xsdInteger        = "http://www.w3.org/2001/XMLSchema#integer"
)

/**
 * A struct for holding data about a given track
 */
type Track struct {
	Fingerprint string            `json:"fingerprint"`
	Duration    int               `json:"duration"`
	URL         string            `json:"url"`
	ID          int               `json:"trackid"`
	Tags        map[string]string `json:"tags"`
	Weighting   float64           `json:"weighting"`
}



func getSearchUrl(predicateID string, value string, mediaMetadataManagerOrigin string) rdf2go.Term {
	return rdf2go.NewResource(fmt.Sprintf("%s/search?p.%s=%s", mediaMetadataManagerOrigin, predicateID, url.QueryEscape(value)))
}

// resolvePredicateURI expands a predicate URI from the registry.
// URIs starting with "/" are relative to APP_ORIGIN and are prefixed at runtime.
// All other URIs are used as-is.
func resolvePredicateURI(predicateURI string, appOrigin string) string {
	if strings.HasPrefix(predicateURI, "/") {
		return appOrigin + predicateURI
	}
	return predicateURI
}

// Map a predicate/value/uri triplet into predicate URI + list of RDF objects.
// uri is the value from the tag's uri column (may be nil/empty for legacy data);
// for predicates that produce IRI objects, uri takes precedence over value when set.
// appOrigin is used for vocabulary URIs (ontology predicates);
// mediaMetadataManagerOrigin is used for resource URIs (tracks, albums, search).
func mapPredicate(predicateID string, value string, uri *string, mediaMetadataManagerOrigin string, appOrigin string) (string, []rdf2go.Term) {

	// Dispatch on PredicateConfig for all explicitly registered predicates
	// (Literal, URIObject, SearchURL, MBIDPrefix, and explicitly-omitted ones like lastSuccessfulPlay).
	if rdfConfig, ok := predicateconfig.Get(predicateID); ok {
		switch rdfConfig.ValueShape {
		case predicateconfig.ValueShapeLiteral:
			predicateURI := resolvePredicateURI(rdfConfig.PredicateURI, appOrigin)
			return predicateURI, []rdf2go.Term{rdf2go.NewLiteral(value)}
		case predicateconfig.ValueShapeURIObject:
			predicateURI := resolvePredicateURI(rdfConfig.PredicateURI, appOrigin)
			if uri == nil || *uri == "" {
				return "", nil // skip tags with no URI — value alone is not a valid IRI
			}
			return predicateURI, []rdf2go.Term{rdf2go.NewResource(*uri)}
		case predicateconfig.ValueShapeSearchURL:
			predicateURI := resolvePredicateURI(rdfConfig.PredicateURI, appOrigin)
			return predicateURI, []rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}
		case predicateconfig.ValueShapeMBIDPrefix:
			predicateURI := resolvePredicateURI(rdfConfig.PredicateURI, appOrigin)
			return predicateURI, []rdf2go.Term{rdf2go.NewResource(rdfConfig.URIPrefix + value)}
		case predicateconfig.ValueShapeOmit:
			// Explicitly suppressed from RDF output (e.g. lastSuccessfulPlay).
			return "", nil
		default:
			return "", nil // unknown shape — omit rather than emitting a potentially wrong triple
		}
	}

	// Predicate not in registry — omit rather than emitting an orphan triple.
	// All predicates with in-use data must be explicitly registered above.
	return "", nil
}


// exportRDF runs the query and writes Turtle output to a file
func ExportRDF(dbPath, outFile string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT t.id, t.url, t.duration, tg.predicateid, tg.value, tg.uri
		FROM track t
		LEFT JOIN tag tg ON tg.trackid = t.id
		ORDER BY t.id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	albumRows, err := db.Query(`SELECT id, name FROM album ORDER BY id`)
	if err != nil {
		return err
	}
	defer albumRows.Close()

	ontologyGraph, err := OntologyToRdf()
	if err != nil {
		return err
	}
	g, err := TrackToRdf(rows)
	if err != nil {
		return err
	}
	albumGraph, err := AlbumToRdf(albumRows)
	if err != nil {
		return err
	}
	g.Merge(albumGraph)
	g.Merge(ontologyGraph)

	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return g.Serialize(f, "turtle")
}
func TrackToRdf(rows *sql.Rows) (*rdf2go.Graph, error) {
	mediaMetadataManagerOrigin := os.Getenv("MEDIA_METADATA_MANAGER_ORIGIN")
	appOrigin := os.Getenv("APP_ORIGIN")
	g := rdf2go.NewGraph("")
	moTrack := rdf2go.NewResource("http://purl.org/ontology/mo/Track")
	g.AddTriple(moTrack,
		rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		rdf2go.NewResource("http://www.w3.org/2002/07/owl#Class"),
	)
	g.AddTriple(moTrack,
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("Track", "en"),
	)
	g.AddTriple(
		moTrack,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/hasCategory"),
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Musical"),
	)
	g.AddTriple(
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Musical"),
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("Musical", "en"),
	)
	var lastTrackID int
	var subject rdf2go.Term

	for rows.Next() {
		var urlStr string
		var trackID int
		var duration *int
		var predicateID, value, uri *string

		if err := rows.Scan(&trackID, &urlStr, &duration, &predicateID, &value, &uri); err != nil {
			return g, err
		}

		if trackID != lastTrackID {
			subject = rdf2go.NewResource(fmt.Sprintf("%s/tracks/%d", mediaMetadataManagerOrigin, trackID))
			g.AddTriple(subject,
				rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
				rdf2go.NewResource("http://purl.org/ontology/mo/Track"))

			if urlStr != "" {
				g.AddTriple(subject, rdf2go.NewResource("http://purl.org/dc/terms/identifier"),
					rdf2go.NewLiteral(urlStr))
			}
			if duration != nil {
				durLiteral := rdf2go.NewLiteralWithDatatype(
					"PT"+strconv.Itoa(*duration)+"S",
					rdf2go.NewResource("http://www.w3.org/2001/XMLSchema#duration"),
				)
				g.AddTriple(subject, rdf2go.NewResource("http://purl.org/ontology/mo/duration"), durLiteral)
			}
			lastTrackID = trackID
		}

		if predicateID != nil && value != nil {
			predURI, objs := mapPredicate(*predicateID, *value, uri, mediaMetadataManagerOrigin, appOrigin)
			for _, obj := range objs {
				g.AddTriple(subject, rdf2go.NewResource(predURI), obj)
			}
		}
	}

	return g, nil
}
// AlbumToRdf converts rows from the album table into an RDF graph.
// Each row must have columns: id (int), name (string).
// Emits rdf:type mo:Record and skos:prefLabel for each album,
// plus type-level metadata for mo:Record itself so the document is self-contained.
func AlbumToRdf(rows *sql.Rows) (*rdf2go.Graph, error) {
	mediaMetadataManagerOrigin := os.Getenv("MEDIA_METADATA_MANAGER_ORIGIN")
	g := rdf2go.NewGraph("")
	moRecord := rdf2go.NewResource("http://purl.org/ontology/mo/Record")
	g.AddTriple(moRecord,
		rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		rdf2go.NewResource("http://www.w3.org/2002/07/owl#Class"),
	)
	g.AddTriple(moRecord,
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("Album", "en"),
	)
	g.AddTriple(
		moRecord,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/hasCategory"),
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Musical"),
	)
	g.AddTriple(
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Musical"),
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("Musical", "en"),
	)

	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return g, err
		}
		subject := rdf2go.NewResource(fmt.Sprintf("%s/albums/%d", mediaMetadataManagerOrigin, id))
		g.AddTriple(subject,
			rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
			rdf2go.NewResource("http://purl.org/ontology/mo/Record"))
		g.AddTriple(subject,
			rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
			rdf2go.NewLiteral(name))
	}

	return g, nil
}

func OntologyToRdf() (*rdf2go.Graph, error) {
	appOrigin := os.Getenv("APP_ORIGIN")
	if appOrigin == "" {
		return nil, fmt.Errorf("APP_ORIGIN not set")
	}

	ontologyURI := appOrigin + "/ontology"
	g := rdf2go.NewGraph(ontologyURI)

	// Define some common URIs
	owlDatatypeProperty := rdf2go.NewResource("http://www.w3.org/2002/07/owl#DatatypeProperty")
	owlObjectProperty := rdf2go.NewResource("http://www.w3.org/2002/07/owl#ObjectProperty")
	moTrack := rdf2go.NewResource("http://purl.org/ontology/mo/Track")

	// Add ontology metadata first
	ontologyRes := rdf2go.NewResource(ontologyURI)
	g.AddTriple(ontologyRes,
		rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		rdf2go.NewResource("http://www.w3.org/2002/07/owl#Ontology"))
	g.AddTriple(ontologyRes,
		rdf2go.NewResource("http://purl.org/dc/terms/title"),
		rdf2go.NewLiteralWithLanguage("Media Metadata Manager Ontology", "en"))
	g.AddTriple(ontologyRes,
		rdf2go.NewResource("http://purl.org/dc/terms/description"),
		rdf2go.NewLiteral("An ontology defining custom properties used by the Media Metadata Manager RDF exporter."))

	// Helper for adding a property definition
	addProperty := func(localName, label string, propertyType rdf2go.Term, rangeURI rdf2go.Term, comment string, inverse string, inverseLabel string) {
		subject := rdf2go.NewResource(fmt.Sprintf("%s#%s", ontologyURI, localName))
		g.AddTriple(subject,
			rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
			propertyType)
		g.AddTriple(subject,
			rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
			rdf2go.NewLiteralWithLanguage(label, "en"))
		g.AddTriple(subject,
			rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#domain"),
			moTrack)
		g.AddTriple(subject,
			rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#range"),
			rangeURI)
		g.AddTriple(subject,
			rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#comment"),
			rdf2go.NewLiteral(comment))
		if inverse != "" {
			inverse_uri := rdf2go.NewResource(fmt.Sprintf("%s#%s", ontologyURI, inverse))
			g.AddTriple(subject,
				rdf2go.NewResource("http://www.w3.org/2002/07/owl#inverseOf"),
				inverse_uri)
			g.AddTriple(inverse_uri,
				rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
				propertyType)
			g.AddTriple(inverse_uri,
				rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
				rdf2go.NewLiteralWithLanguage(inverseLabel, "en"))
		}
	}

	// Define all custom ontology properties from mapPredicate
	addProperty("dateAdded", "Date added", owlDatatypeProperty,
		rdf2go.NewResource("http://www.w3.org/2001/XMLSchema#dateTime"),
		"The date the track was added to the collection.", "", "")

	addProperty("trigger", "Trigger (offence)", owlObjectProperty,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Offence"),
		"Any potential triggers or offence covered by the track's subject matter.", "", "")

	addProperty("memory", "Memory", owlObjectProperty,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Memory"),
		"What this song reminds Luke of.", "", "")

	addProperty("soundtrack", "Soundtrack", owlObjectProperty,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/CreativeWork"),
		"Creative Work whose soundtrack this track appears in.", "", "")

	addProperty("theme_tune", "Theme tune", owlObjectProperty,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/CreativeWork"),
		"Creative Work this track is the primary theme tune of.", "", "")

	addProperty("availability", "Availability", owlObjectProperty,
		rdf2go.NewResource(fmt.Sprintf("%s#availabilityScheme", ontologyURI)),
		"How easy it would be to replace this track if something happened to my collection.", "", "")

	addProperty("about", "About", owlObjectProperty,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"Concepts which are the primary topic of this track.", "subjectOf", "Subject Of")

	addProperty("mentions", "Mentions", owlObjectProperty,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"Concepts which are mentioned or alluded to by this track.", "mentionedBy", "Mentioned By")

	// trackLanguage: the scoped predicate for track→language relationships.
	// The inverse trackInLanguage is materialised by the arachne ingestor via
	// owl:inverseOf, enabling "Tracks in this language" sections on Language pages.
	// Range is the eolas language collection URI (individual language URIs are
	// sub-resources, e.g. https://eolas.l42.eu/metadata/language/gd/).
	addProperty("trackLanguage", "Track Language", owlObjectProperty,
		rdf2go.NewResource("https://eolas.l42.eu/metadata/language/"),
		"The language a track is performed in.",
		"trackInLanguage", "Tracks in this language")

	// singalong and dance properties (newly promoted from Omit to URIObject).
	addProperty("singalong", "Singalong", owlObjectProperty,
		rdf2go.NewResource(fmt.Sprintf("%s#singalongScheme", ontologyURI)),
		"How singable this track is — an ordinal scale from 0 (can't sing along) to 5 (fully a cappella).", "", "")

	addProperty("dance", "Dance", owlObjectProperty,
		rdf2go.NewResource(fmt.Sprintf("%s#danceScheme", ontologyURI)),
		"The style of dance which goes with this track.", "", "")

	// Ordinal level properties for availability and singalong.
	// Each skos:Concept in these schemes carries an integer level via these properties,
	// enabling consumers to derive the ordinal ordering without SKOS traversal.
	availabilityLevelProp := rdf2go.NewResource(fmt.Sprintf("%s#availabilityLevel", ontologyURI))
	g.AddTriple(availabilityLevelProp, rdf2go.NewResource(rdfType), owlDatatypeProperty)
	g.AddTriple(availabilityLevelProp,
		rdf2go.NewResource(skosPrefLabel),
		rdf2go.NewLiteralWithLanguage("Availability Level", "en"))
	g.AddTriple(availabilityLevelProp,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#range"),
		rdf2go.NewResource(xsdInteger))
	g.AddTriple(availabilityLevelProp,
		rdf2go.NewResource(rdfsComment),
		rdf2go.NewLiteral("Ordinal level for availability concepts. 0 = I have the canonical copy (hardest to replace); 4 = Ubiquitous (easiest to replace). Lower level = more unique."))

	singalongLevelProp := rdf2go.NewResource(fmt.Sprintf("%s#singalongLevel", ontologyURI))
	g.AddTriple(singalongLevelProp, rdf2go.NewResource(rdfType), owlDatatypeProperty)
	g.AddTriple(singalongLevelProp,
		rdf2go.NewResource(skosPrefLabel),
		rdf2go.NewLiteralWithLanguage("Singalong Level", "en"))
	g.AddTriple(singalongLevelProp,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#range"),
		rdf2go.NewResource(xsdInteger))
	g.AddTriple(singalongLevelProp,
		rdf2go.NewResource(rdfsComment),
		rdf2go.NewLiteral("Ordinal level for singalong concepts. 0 = No chance; 5 = Can do it a cappella without a lyric sheet."))

	// Helper for adding a SKOS concept scheme with its concepts.
	// schemeName is the fragment identifier (e.g. "provenanceScheme").
	// schemeLabel is the human-readable English label.
	// levelPropURI is the URI of the ordinal level property, or "" if not ordinal.
	addSKOSScheme := func(predicate, schemeName, schemeLabel, schemeComment, levelPropURI string) {
		schemeURI := rdf2go.NewResource(fmt.Sprintf("%s#%s", ontologyURI, schemeName))
		g.AddTriple(schemeURI, rdf2go.NewResource(rdfType), rdf2go.NewResource(skosConceptScheme))
		g.AddTriple(schemeURI, rdf2go.NewResource(skosPrefLabel), rdf2go.NewLiteralWithLanguage(schemeLabel, "en"))
		if schemeComment != "" {
			g.AddTriple(schemeURI, rdf2go.NewResource(rdfsComment), rdf2go.NewLiteral(schemeComment))
		}

		for _, c := range predicateconfig.GetSKOSConcepts(predicate) {
			conceptURI := rdf2go.NewResource(predicateconfig.ConceptURI(appOrigin, predicate, c.Slug))
			g.AddTriple(conceptURI, rdf2go.NewResource(rdfType), rdf2go.NewResource(skosConcept))
			g.AddTriple(conceptURI, rdf2go.NewResource(skosPrefLabel), rdf2go.NewLiteralWithLanguage(c.PrefLabel, "en"))
			g.AddTriple(conceptURI, rdf2go.NewResource(skosNotation), rdf2go.NewLiteral(c.Slug))
			g.AddTriple(conceptURI, rdf2go.NewResource(skosInScheme), schemeURI)
			if levelPropURI != "" {
				g.AddTriple(conceptURI,
					rdf2go.NewResource(levelPropURI),
					rdf2go.NewLiteralWithDatatype(strconv.Itoa(c.Level), rdf2go.NewResource(xsdInteger)))
			}
		}
	}

	addSKOSScheme("provenance", "provenanceScheme", "Provenance Scheme", "", "")
	addSKOSScheme("availability", "availabilityScheme", "Availability Scheme",
		"Ordinal scheme for track availability. Use the ontology#availabilityLevel property on each concept to retrieve the ordinal integer.",
		fmt.Sprintf("%s#availabilityLevel", ontologyURI))
	addSKOSScheme("singalong", "singalongScheme", "Singalong Scheme",
		"Ordinal scheme for how singable a track is. Use the ontology#singalongLevel property on each concept to retrieve the ordinal integer.",
		fmt.Sprintf("%s#singalongLevel", ontologyURI))
	addSKOSScheme("dance", "danceScheme", "Dance Scheme",
		// Dance eolas-future migration path (rdfs:comment on the scheme):
		// When eolas:Dance entities are ready, create one per concept using the same slug as
		// canonical key. Flip registry AllowedOrigins to OriginEolas. Backfill tag.uri by slug
		// lookup. The slug is the load-bearing identifier — not the URI string — so the
		// migration is mechanical: one lookup per tag row, no churn.
		"Dance style vocabulary. Slug-based URIs are intentional: a future migration to first-class eolas:Dance entities shares the same slug as canonical lookup key, making the URI rewrite mechanical. See issue #265 (consolidated into #258).",
		"")

	g.AddTriple(
		rdf2go.NewResource(appOrigin + "/ontology#about"),
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#subPropertyOf"),
		rdf2go.NewResource(appOrigin + "/ontology#mentions"),
	)

	// mo:Record class metadata — albums use this type
	moRecord := rdf2go.NewResource("http://purl.org/ontology/mo/Record")
	g.AddTriple(moRecord,
		rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		rdf2go.NewResource("http://www.w3.org/2002/07/owl#Class"))
	g.AddTriple(moRecord,
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("Album", "en"))
	g.AddTriple(moRecord,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/hasCategory"),
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Musical"))

	// onAlbum property: track→album, declared as owl:inverseOf mo:track.
	// Note: mo:track is an external URI (not in our custom ontology namespace) so it
	// cannot be handled by the addProperty helper above. Any owl:inverseOf declaration
	// for an external property MUST also emit a skos:prefLabel for that inverse predicate,
	// as the addProperty helper does automatically for inverses in our own namespace.
	onAlbum := rdf2go.NewResource(ontologyURI + "#onAlbum")
	g.AddTriple(onAlbum,
		rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		owlObjectProperty)
	g.AddTriple(onAlbum,
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("On Album", "en"))
	g.AddTriple(onAlbum,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#domain"),
		moTrack)
	g.AddTriple(onAlbum,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#range"),
		moRecord)
	moTrackProperty := rdf2go.NewResource("http://purl.org/ontology/mo/track")
	g.AddTriple(onAlbum,
		rdf2go.NewResource("http://www.w3.org/2002/07/owl#inverseOf"),
		moTrackProperty)
	g.AddTriple(moTrackProperty,
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("Track", "en"))

	return g, nil
}
