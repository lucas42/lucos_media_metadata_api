package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/deiu/rdf2go"
)

const MEDIA_MANAGER_BASE = "https://media-metadata.l42.eu/"

// Helper: split CSV values and trim whitespace
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func getSearchUrl(predicateID, value string) (rdf2go.Term) {
	return rdf2go.NewResource(fmt.Sprintf("%ssearch?p.%s=%s", MEDIA_MANAGER_BASE, predicateID, url.QueryEscape(value)))
}

// Map a predicate/value pair into predicate URI + list of RDF objects
func mapPredicate(predicateID, value string) (string, []rdf2go.Term) {
	switch predicateID {

	// Date added
	case "added":
		return "http://purl.org/dc/terms/dateAccepted",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	// Title
	case "title":
		return "http://purl.org/dc/terms/title",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	// Artist name (text literal, unless you mint URIs)
	case "artist":
		return "http://purl.org/dc/terms/creator",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	// Album
	case "album":
		return "http://purl.org/dc/terms/isPartOf",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	// Genre
	case "genre":
		return "http://purl.org/dc/terms/subject",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	// Composer: CSV list of names
	case "composer":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v))
			}
		}
		return "http://purl.org/dc/terms/contributor", terms

	// Producer: CSV list
	case "producer":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v))
			}
		}
		return "http://purl.org/dc/terms/contributor", terms

	// Language: CSV of ISO codes → resources (Lexvo)
	case "language":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, rdf2go.NewResource("http://lexvo.org/id/iso639-3/"+v))
			}
		}
		return "http://purl.org/dc/terms/language", terms

	// Offence / trigger warnings
	case "offence":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v))
			}
		}
		return "http://purl.org/dc/terms/subject", terms

	// MusicBrainz IDs
	case "mbid_artist":
		return "http://purl.org/dc/terms/creator",
			[]rdf2go.Term{rdf2go.NewResource("https://musicbrainz.org/artist/" + value)}

	case "mbid_recording":
		return "http://purl.org/dc/terms/identifier",
			[]rdf2go.Term{rdf2go.NewResource("https://musicbrainz.org/recording/" + value)}

	case "mbid_release":
		return "http://purl.org/dc/terms/isPartOf",
			[]rdf2go.Term{rdf2go.NewResource("https://musicbrainz.org/release/" + value)}

	// Comment
	case "comment":
		return "http://schema.org/comment",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	// Lyrics
	case "lyrics":
		return "http://purl.org/ontology/mo/lyrics",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	// Rating
	case "rating":
		return "http://schema.org/ratingValue",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	// Provenance
	case "provenance":
		return "http://purl.org/dc/terms/source",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	// Memory
	case "memory":
		return MEDIA_MANAGER_BASE+"ontology/memory",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	// Soundtrack
	case "soundtrack":
		return "http://purl.org/ontology/mo/soundtrack",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	// Theme tune
	case "theme_tune":
		return "http://purl.org/ontology/mo/theme",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	// Year
	case "year":
		return "http://purl.org/dc/terms/date",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	// Availability
	case "availability":
		return MEDIA_MANAGER_BASE+"ontology/availability",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	default:
		return MEDIA_MANAGER_BASE+"ontology/" + predicateID,
			[]rdf2go.Term{rdf2go.NewLiteral(value)}
	}
}

// RDFHandler outputs all tracks and tags as RDF/Turtle
func (store Datastore) RDFHandler(w http.ResponseWriter, r *http.Request) {
	g := rdf2go.NewGraph("")

	rows, err := store.DB.Query(`
		SELECT t.id, t.url, t.duration, tg.predicateid, tg.value
		FROM track t
		LEFT JOIN tag tg ON tg.trackid = t.id
		ORDER BY t.id
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lastTrackID int
	var subject rdf2go.Term

	for rows.Next() {
		var url string
		var trackID int
		var duration *int
		var predicateID, value *string

		if err := rows.Scan(&trackID, &url, &duration, &predicateID, &value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if trackID != lastTrackID {
			subject = rdf2go.NewResource(fmt.Sprintf("%stracks/%d", MEDIA_MANAGER_BASE, trackID))
			g.AddTriple(subject,
				rdf2go.NewResource("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
				rdf2go.NewResource("http://purl.org/ontology/mo/Track"))

			if url != "" {
				g.AddTriple(subject, rdf2go.NewResource("http://purl.org/dc/terms/identifier"),
					rdf2go.NewResource(url))
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
			predURI, objs := mapPredicate(*predicateID, *value)
			for _, obj := range objs {
				g.AddTriple(subject, rdf2go.NewResource(predURI), obj)
			}
		}
	}

	w.Header().Set("Content-Type", "text/turtle")
	w.WriteHeader(http.StatusOK)
	if err := g.Serialize(w, "turtle"); err != nil {
		http.Error(w, "serialization failed", http.StatusInternalServerError)
		return
	}
}
