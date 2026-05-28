// migrate_skos_tags populates the uri column for provenance, availability, singalong, and dance
// tags whose uri is currently empty, mapping each existing name/value to its SKOS concept URI.
//
// Background: these four predicates have been migrated from ValueShapeSearchURL (provenance,
// availability) or ValueShapeOmit (singalong, dance) to ValueShapeURIObject backed by a
// local SKOS concept scheme at {APP_ORIGIN}/vocab/{predicate}/{slug}. Existing tag rows
// carry the old bare-string name in the value column but have no uri. This script fills in
// the uri column so that the API's write-time validation continues to pass and the RDF
// exporter can emit proper IRI triples.
//
// Run this script as part of the deploy release window:
//
//  1. Deploy the updated API (with the new registry entries) first so the write path
//     accepts URIs for these predicates going forward.
//  2. Run the migration via Docker (the binary is included in the API image):
//       docker run --rm \
//         -v lucos_media_metadata_api_db:/var/lib/media-metadata/ \
//         lucas42/lucos_media_metadata_api \
//         migrate_skos_tags \
//         -db /var/lib/media-metadata/media.sqlite \
//         -app-origin https://media.l42.eu
//  3. Verify: run with -dry-run first to inspect what will be updated.
//
// What the script does (per predicate):
//   provenance:  values are slugs (e.g. "7digital", "bandcamp") → resolved directly by slug.
//   availability: values are ordinal integer strings ("0"–"4") → matched by Level field.
//   singalong:   values are ordinal integer strings ("0"–"5") → matched by Level field.
//   dance:       values are display names (e.g. "Lindy Hop") → resolved by prefLabel (case-insensitive).
//
// The script is idempotent: rows that already have a non-empty uri are skipped.
// Values that don't map to any known concept are reported and left unmodified (manual review).

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"lucos_media_metadata_api/predicateconfig"
)

func main() {
	dbPath := flag.String("db", "", "Path to the SQLite database file (required)")
	appOrigin := flag.String("app-origin", "", "APP_ORIGIN base URL, e.g. https://media.l42.eu (required)")
	dryRun := flag.Bool("dry-run", false, "Print what would be done without making any changes")
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "error: -db is required")
		flag.Usage()
		os.Exit(1)
	}
	if *appOrigin == "" {
		fmt.Fprintln(os.Stderr, "error: -app-origin is required")
		flag.Usage()
		os.Exit(1)
	}

	origin := strings.TrimSuffix(*appOrigin, "/")

	db, err := sqlx.Open("sqlite3", *dbPath+"?_busy_timeout=10000")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.MustExec("PRAGMA journal_mode=WAL;")
	db.MustExec("PRAGMA foreign_keys = ON;")

	predicates := []string{"provenance", "availability", "singalong", "dance"}

	totalUpdated := int64(0)
	totalSkipped := 0

	for _, predicate := range predicates {
		updated, skipped, err := migratePredicate(db, origin, predicate, *dryRun)
		if err != nil {
			log.Fatalf("migrate %s: %v", predicate, err)
		}
		totalUpdated += updated
		totalSkipped += skipped
	}

	if *dryRun {
		fmt.Printf("\n[dry-run] No changes made. Would update %d row(s); %d unresolvable value(s) left for manual review.\n", totalUpdated, totalSkipped)
	} else {
		fmt.Printf("\nMigration complete. %d row(s) updated; %d unresolvable value(s) left for manual review.\n", totalUpdated, totalSkipped)
	}
}

// migratePredicate handles the SKOS URI migration for a single predicate.
// Returns the number of rows updated and the number of unresolvable values.
func migratePredicate(db *sqlx.DB, appOrigin, predicate string, dryRun bool) (int64, int, error) {
	// Fetch distinct values with no URI set.
	var values []string
	err := db.Select(&values, `
		SELECT DISTINCT value FROM tag
		WHERE predicateid = ? AND (uri IS NULL OR uri = '')
		ORDER BY value
	`, predicate)
	if err != nil {
		return 0, 0, fmt.Errorf("query values: %w", err)
	}

	if len(values) == 0 {
		fmt.Printf("[%s] No unmigrated rows found.\n", predicate)
		return 0, 0, nil
	}

	fmt.Printf("[%s] Found %d distinct unmigrated value(s):\n", predicate, len(values))

	// Build value → URI map.
	valueToURI := make(map[string]string, len(values))
	unresolvable := []string{}

	concepts := predicateconfig.GetSKOSConcepts(predicate)

	for _, val := range values {
		uri := resolveValue(appOrigin, predicate, val, concepts)
		if uri == "" {
			fmt.Printf("  [%s] %q → UNRESOLVABLE (will skip)\n", predicate, val)
			unresolvable = append(unresolvable, val)
		} else {
			fmt.Printf("  [%s] %q → %s\n", predicate, val, uri)
			valueToURI[val] = uri
		}
	}

	if dryRun {
		return int64(len(valueToURI)), len(unresolvable), nil
	}

	if len(valueToURI) == 0 {
		return 0, len(unresolvable), nil
	}

	// Apply updates in a single transaction.
	tx, err := db.Beginx()
	if err != nil {
		return 0, 0, fmt.Errorf("begin transaction: %w", err)
	}

	totalUpdated := int64(0)
	for val, uri := range valueToURI {
		res, err := tx.Exec(`
			UPDATE tag SET uri = ?
			WHERE predicateid = ? AND value = ? AND (uri IS NULL OR uri = '')
		`, uri, predicate, val)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, fmt.Errorf("update tag rows for value %q: %w", val, err)
		}
		rows, _ := res.RowsAffected()
		totalUpdated += rows
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit: %w", err)
	}

	return totalUpdated, len(unresolvable), nil
}

// resolveValue maps a tag value to its SKOS concept URI for the given predicate.
// Returns an empty string if no concept matches.
func resolveValue(appOrigin, predicate, value string, concepts []predicateconfig.SKOSConcept) string {
	if len(concepts) == 0 {
		return ""
	}

	// For availability and singalong: existing values are ordinal integers ("0", "1", ...).
	// Match by Level field.
	if predicate == "availability" || predicate == "singalong" {
		level, err := strconv.Atoi(value)
		if err == nil {
			for _, c := range concepts {
				if c.Level == level {
					return predicateconfig.ConceptURI(appOrigin, predicate, c.Slug)
				}
			}
		}
	}

	// For all predicates: try exact slug match first.
	for _, c := range concepts {
		if c.Slug == value {
			return predicateconfig.ConceptURI(appOrigin, predicate, c.Slug)
		}
	}

	// Fallback: case-insensitive prefLabel match (handles dance display names like "Lindy Hop").
	lower := strings.ToLower(value)
	for _, c := range concepts {
		if strings.ToLower(c.PrefLabel) == lower {
			return predicateconfig.ConceptURI(appOrigin, predicate, c.Slug)
		}
	}

	return ""
}
