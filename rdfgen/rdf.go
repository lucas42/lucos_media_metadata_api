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



// Helper: split CSV values and trim whitespace
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func getSearchUrl(predicateID string, value string, mediaMetadataManagerOrigin string) rdf2go.Term {
	return rdf2go.NewResource(fmt.Sprintf("%s/search?p.%s=%s", mediaMetadataManagerOrigin, predicateID, url.QueryEscape(value)))
}

// Map a predicate/value pair into predicate URI + list of RDF objects
func mapPredicate(predicateID string, value string, mediaMetadataManagerOrigin string) (string, []rdf2go.Term) {
	switch predicateID {

	case "added":
		return mediaMetadataManagerOrigin + "/ontology#dateAdded",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "title":
		// Uses skos:prefLabel for consistency with other items in the triplestore
		// But might be useful to also add a dc:title predicate too.
		return "http://www.w3.org/2004/02/skos/core#prefLabel",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "artist":
		return "http://xmlns.com/foaf/0.1/maker",
			[]rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}

	case "album":
		return "http://purl.org/dc/terms/isPartOf",
			[]rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}

	case "genre":
		return "http://purl.org/ontology/mo/genre",
			[]rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}

	case "composer":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v, mediaMetadataManagerOrigin))
			}
		}
		// Technically the music ontology says a composer is of a composition, rather than the track.
		// But trying to model all that is a faff.  Here we misuse the composer predicate for simplicity.
		return "http://purl.org/ontology/mo/composer", terms

	case "producer":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v, mediaMetadataManagerOrigin))
			}
		}
		return "http://purl.org/ontology/mo/producer", terms

	case "language":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				// Currently the DB has a mix of iso639-1, iso639-3, iso639-6 and custom x- language codes
				terms = append(terms, rdf2go.NewResource(fmt.Sprintf("https://eolas.l42.eu/metadata/language/%s/", v)))
			}
		}
		return "http://purl.org/dc/terms/language", terms

	case "offence":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v, mediaMetadataManagerOrigin))
			}
		}
		return mediaMetadataManagerOrigin + "/ontology#trigger", terms

	case "mbid_artist":
		return "http://purl.org/dc/terms/creator",
			[]rdf2go.Term{rdf2go.NewResource("https://musicbrainz.org/artist/" + value)}

	case "mbid_recording":
		return "http://purl.org/dc/terms/identifier",
			[]rdf2go.Term{rdf2go.NewResource("https://musicbrainz.org/recording/" + value)}

	case "mbid_release":
		return "http://purl.org/dc/terms/isPartOf",
			[]rdf2go.Term{rdf2go.NewResource("https://musicbrainz.org/release/" + value)}

	case "comment":
		return "http://schema.org/comment",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "lyrics":
		// Technically, the music ontology uses mo:lyrics to link to a mo:Lyrics node, which then has mo:text to the literal lyrics.
		// But for now, just misuse the relationship straight to the literal
		return "http://purl.org/ontology/mo/lyrics",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "rating":
		// Technically, this should be an attribute of a sdo:Rating class (which can explain what the rating is out of etc)
		// But for now, just misuse it by placing directly on the Track
		return "http://schema.org/ratingValue",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "provenance":
		return "http://purl.org/dc/terms/source",
			[]rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}

	case "memory":
		return mediaMetadataManagerOrigin + "/ontology#memory",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "soundtrack":
		return mediaMetadataManagerOrigin + "/ontology#soundtrack",
			[]rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}

	case "theme_tune":
		// This should be treated as a subClass of soundtrack.  But rather than adding both predicates here, best to specify that in the ontology and apply inferencing
		return mediaMetadataManagerOrigin + "/ontology#theme_tune",
			[]rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}

	case "year":
		return "http://purl.org/dc/terms/date",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "availability":
		return mediaMetadataManagerOrigin + "/ontology#availability",
			[]rdf2go.Term{getSearchUrl(predicateID, value, mediaMetadataManagerOrigin)}

	case "about":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			terms = append(terms, rdf2go.NewResource(v))
		}
		return mediaMetadataManagerOrigin + "/ontology#about", terms

	case "mentions":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			terms = append(terms, rdf2go.NewResource(v))
		}
		return mediaMetadataManagerOrigin + "/ontology#mentions", terms


	default:
		return mediaMetadataManagerOrigin + "/ontology#" + predicateID,
			[]rdf2go.Term{rdf2go.NewLiteral(value)}
	}
}


// exportRDF runs the query and writes Turtle output to a file
func ExportRDF(dbPath, outFile string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT t.id, t.url, t.duration, tg.predicateid, tg.value
		FROM track t
		LEFT JOIN tag tg ON tg.trackid = t.id
		ORDER BY t.id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	ontologyGraph, err := OntologyToRdf()
	if err != nil {
		return err
	}
	g, err := TrackToRdf(rows)
	if err != nil {
		return err
	}
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
	g := rdf2go.NewGraph("")
	g.AddTriple(rdf2go.NewResource("http://purl.org/ontology/mo/Track"),
		rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		rdf2go.NewResource("http://www.w3.org/2002/07/owl#Class"),
	)
	g.AddTriple(rdf2go.NewResource("http://purl.org/ontology/mo/Track"),
		rdf2go.NewResource("http://www.w3.org/2004/02/skos/core#prefLabel"),
		rdf2go.NewLiteralWithLanguage("Track", "en"),
	)
	var lastTrackID int
	var subject rdf2go.Term

	for rows.Next() {
		var urlStr string
		var trackID int
		var duration *int
		var predicateID, value *string

		if err := rows.Scan(&trackID, &urlStr, &duration, &predicateID, &value); err != nil {
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
			predURI, objs := mapPredicate(*predicateID, *value, mediaMetadataManagerOrigin)
			for _, obj := range objs {
				g.AddTriple(subject, rdf2go.NewResource(predURI), obj)
			}
		}
	}

	return g, nil
}
func OntologyToRdf() (*rdf2go.Graph, error) {
	mediaMetadataManagerOrigin := os.Getenv("MEDIA_METADATA_MANAGER_ORIGIN")
	if mediaMetadataManagerOrigin == "" {
		return nil, fmt.Errorf("MEDIA_METADATA_MANAGER_ORIGIN not set")
	}

	ontologyURI := mediaMetadataManagerOrigin + "/ontology"
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
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"Any potential triggers or offence covered by the track's subject matter.", "", "")

	addProperty("memory", "Memory", owlObjectProperty,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Memory"),
		"What this song reminds Luke of.", "", "")

	addProperty("soundtrack", "Soundtrack", owlObjectProperty,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"Creative Work whose soundtrack this track appears in.", "", "")

	addProperty("theme_tune", "Theme tune", owlObjectProperty,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"Creative Work this track is the primary theme tune of.", "", "")

	addProperty("availability", "Availability", owlObjectProperty,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"How easy it would be to replace this track if something happened to my collection.", "", "")

	addProperty("about", "About", owlObjectProperty,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"Concepts which are the primary topic of this track.", "subjectOf", "Subject Of")

	addProperty("mentions", "Mentions", owlObjectProperty,
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#Resource"),
		"Concepts which are mentioned or alluded to by this track.", "mentionedBy", "Mentioned By")

	g.AddTriple(
		rdf2go.NewResource(mediaMetadataManagerOrigin + "/ontology#about"),
		rdf2go.NewResource("http://www.w3.org/2000/01/rdf-schema#subPropertyOf"),
		rdf2go.NewResource(mediaMetadataManagerOrigin + "/ontology#mentions"),
	)

	g.AddTriple(
		moTrack,
		rdf2go.NewResource("https://eolas.l42.eu/ontology/hasCategory"),
		rdf2go.NewResource("https://eolas.l42.eu/ontology/Musical"),
	)

	return g, nil
}
