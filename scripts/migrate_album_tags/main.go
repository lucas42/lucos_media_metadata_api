// migrate_album_tags converts existing freetext album tag values to album entity
// references.
//
// Run this script as part of the coordinated Phase B/C rollout window:
//
//   1. Ask users to pause edits to album tags.
//   2. Run this script against the production database:
//        go run ./scripts/migrate_album_tags/ -db /var/lib/media-metadata/media.sqlite -origin https://media-metadata.l42.eu
//   3. Deploy the updated API (reads return {name, uri}, writes require uri) and
//      the updated UI together.
//   4. Resume edits.
//
// What the script does:
//   - Reads all distinct freetext album tag values (rows where predicate='album'
//     and uri is empty).
//   - Creates one album entity per distinct name (INSERT OR IGNORE to handle
//     partial runs safely).
//   - Rewrites each tag row: sets value = album name, uri = album URL.
//
// The script is idempotent: rows that already have a non-empty uri are skipped.
// Safe to re-run if interrupted.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "", "Path to the SQLite database file (required)")
	origin := flag.String("origin", "", "APP_ORIGIN base URL, e.g. https://media-metadata.l42.eu (required)")
	dryRun := flag.Bool("dry-run", false, "Print what would be done without making any changes")
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "error: -db is required")
		flag.Usage()
		os.Exit(1)
	}
	if *origin == "" {
		fmt.Fprintln(os.Stderr, "error: -origin is required")
		flag.Usage()
		os.Exit(1)
	}
	appOrigin := strings.TrimSuffix(*origin, "/")

	db, err := sqlx.Open("sqlite3", *dbPath+"?_busy_timeout=10000")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.MustExec("PRAGMA journal_mode=WAL;")
	db.MustExec("PRAGMA foreign_keys = ON;")

	// Collect distinct freetext album values (no uri set yet).
	var freenames []string
	err = db.Select(&freenames, `
		SELECT DISTINCT value FROM tag
		WHERE predicateid = 'album' AND (uri IS NULL OR uri = '')
	`)
	if err != nil {
		log.Fatalf("query freetext album values: %v", err)
	}

	if len(freenames) == 0 {
		fmt.Println("No freetext album tags found; nothing to migrate.")
		return
	}

	fmt.Printf("Found %d distinct freetext album value(s):\n", len(freenames))
	for _, n := range freenames {
		fmt.Printf("  %q\n", n)
	}

	if *dryRun {
		fmt.Println("\n[dry-run] No changes made.")
		return
	}

	// Create album entities and rewrite tags in a single transaction.
	tx, err := db.Beginx()
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}

	for _, name := range freenames {
		// Upsert album (safe if run more than once).
		_, err = tx.Exec(`INSERT OR IGNORE INTO album(name) VALUES(?)`, name)
		if err != nil {
			_ = tx.Rollback()
			log.Fatalf("insert album %q: %v", name, err)
		}

		// Fetch the album id (whether just created or already existing).
		var albumID int
		err = tx.Get(&albumID, `SELECT id FROM album WHERE name = ?`, name)
		if err != nil {
			_ = tx.Rollback()
			log.Fatalf("get album id for %q: %v", name, err)
		}

		albumURL := fmt.Sprintf("%s/v3/albums/%d", appOrigin, albumID)

		// Rewrite all tag rows for this freetext value.
		res, err := tx.Exec(`
			UPDATE tag SET value = ?, uri = ?
			WHERE predicateid = 'album' AND value = ? AND (uri IS NULL OR uri = '')
		`, name, albumURL, name)
		if err != nil {
			_ = tx.Rollback()
			log.Fatalf("update tags for album %q: %v", name, err)
		}
		rowsAffected, _ := res.RowsAffected()
		fmt.Printf("  Album %q (id=%d) → %d tag row(s) updated\n", name, albumID, rowsAffected)
	}

	if err = tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("\nMigration complete. %d album entity/entities created or confirmed.\n", len(freenames))
}
