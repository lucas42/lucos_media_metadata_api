package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
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

func getSearchUrl(predicateID, value string) rdf2go.Term {
	return rdf2go.NewResource(fmt.Sprintf("%ssearch?p.%s=%s", MEDIA_MANAGER_BASE, predicateID, url.QueryEscape(value)))
}

// Map a predicate/value pair into predicate URI + list of RDF objects
func mapPredicate(predicateID, value string) (string, []rdf2go.Term) {
	switch predicateID {

	case "added":
		return "http://purl.org/dc/terms/dateAccepted",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "title":
		return "http://purl.org/dc/terms/title",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "artist":
		return "http://purl.org/dc/terms/creator",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	case "album":
		return "http://purl.org/dc/terms/isPartOf",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	case "genre":
		return "http://purl.org/dc/terms/subject",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	case "composer":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v))
			}
		}
		return "http://purl.org/dc/terms/contributor", terms

	case "producer":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v))
			}
		}
		return "http://purl.org/dc/terms/contributor", terms

	case "language":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, rdf2go.NewResource("http://lexvo.org/id/iso639-3/"+v))
			}
		}
		return "http://purl.org/dc/terms/language", terms

	case "offence":
		vals := splitCSV(value)
		terms := make([]rdf2go.Term, 0, len(vals))
		for _, v := range vals {
			if v != "" {
				terms = append(terms, getSearchUrl(predicateID, v))
			}
		}
		return "http://purl.org/dc/terms/subject", terms

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
		return "http://purl.org/ontology/mo/lyrics",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "rating":
		return "http://schema.org/ratingValue",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "provenance":
		return "http://purl.org/dc/terms/source",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	case "memory":
		return MEDIA_MANAGER_BASE + "ontology/memory",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "soundtrack":
		return "http://purl.org/ontology/mo/soundtrack",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	case "theme_tune":
		return "http://purl.org/ontology/mo/theme",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	case "year":
		return "http://purl.org/dc/terms/date",
			[]rdf2go.Term{rdf2go.NewLiteral(value)}

	case "availability":
		return MEDIA_MANAGER_BASE + "ontology/availability",
			[]rdf2go.Term{getSearchUrl(predicateID, value)}

	default:
		return MEDIA_MANAGER_BASE + "ontology/" + predicateID,
			[]rdf2go.Term{rdf2go.NewLiteral(value)}
	}
}

// copySQLiteDB creates a temp copy of the SQLite DB file and returns its path
func copySQLiteDB(src string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "sqlite-export")
	if err != nil {
		return "", nil, err
	}
	dst := filepath.Join(tmpDir, "data-export.sqlite")

	srcFile, err := os.Open(src)
	if err != nil {
		return "", nil, err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return "", nil, err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", nil, err
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return dst, cleanup, nil
}

// exportRDF runs the query and writes Turtle output to a file
func exportRDF(dbPath, outFile string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	g := rdf2go.NewGraph("")

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

	var lastTrackID int
	var subject rdf2go.Term

	for rows.Next() {
		var urlStr string
		var trackID int
		var duration *int
		var predicateID, value *string

		if err := rows.Scan(&trackID, &urlStr, &duration, &predicateID, &value); err != nil {
			return err
		}

		if trackID != lastTrackID {
			subject = rdf2go.NewResource(fmt.Sprintf("%stracks/%d", MEDIA_MANAGER_BASE, trackID))
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
			predURI, objs := mapPredicate(*predicateID, *value)
			for _, obj := range objs {
				g.AddTriple(subject, rdf2go.NewResource(predURI), obj)
			}
		}
	}

	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return g.Serialize(f, "turtle")
}

func main() {
	dbPath := os.Getenv("SQLITE_DB_PATH")
	if dbPath == "" {
		log.Fatal("SQLITE_DB_PATH must be set")
	}

	outFile := os.Getenv("RDF_OUTPUT_PATH")
	if dbPath == "" {
		log.Fatal("RDF_OUTPUT_PATH must be set")
	}

	scheduleTracker := os.Getenv("SCHEDULE_TRACKER_ENDPOINT")
	if scheduleTracker == "" {
		log.Fatal("SCHEDULE_TRACKER_ENDPOINT must be set")
	}

	tmpDB, cleanup, err := copySQLiteDB(dbPath)
	if err != nil {
		log.Fatalf("failed to copy db: %v", err)
	}
	defer cleanup()

	if err := exportRDF(tmpDB, outFile); err != nil {
		scheduleTrackerData, _ := json.Marshal(map[string]interface{}{
			"system":    "media_metadata_api_exporter",
			"frequency": 60*60, // 1 hour in seconds
			"status":    "error",
			"message":   err.Error(),
		})
		http.Post(scheduleTracker, "application/json", bytes.NewBuffer(scheduleTrackerData))
		log.Fatalf("failed to export RDF: %v", err)
	}

	log.Printf("RDF export written to %s", outFile)
	scheduleTrackerData, _ := json.Marshal(map[string]interface{}{
		"system":    "media_metadata_api_exporter",
		"frequency": 60*60, // 1 hour in seconds
		"status":    "success",
	})
	http.Post(scheduleTracker, "application/json", bytes.NewBuffer(scheduleTrackerData))
}
