// migrate_memory_tags converts existing freetext memory tag values to Memory
// entity references in lucos_eolas.
//
// Run this script as part of the coordinated release window:
//
//  1. Ask users to pause edits to memory tags.
//  2. Run the migration via Docker (the binary is included in the API image):
//       docker run --rm \
//         -v lucos_media_metadata_api_db:/var/lib/media-metadata/ \
//         lucas42/lucos_media_metadata_api \
//         migrate_memory_tags \
//         -db /var/lib/media-metadata/media.sqlite \
//         -eolas-origin https://eolas.l42.eu \
//         -eolas-key <KEY_LUCOS_EOLAS value>
//  3. Deploy the updated API and lucos_media_metadata_manager together so the
//     form switches to type:search at the same moment the registry starts
//     requiring a URI.
//  4. Resume edits.
//
// What the script does:
//   - Reads all distinct freetext memory tag values (rows where uri is empty).
//   - For each distinct value, looks for an existing Memory entity in eolas by
//     exact case-insensitive name match.
//   - Unmatched values: creates a new Memory entity in eolas.
//   - Rewrites each tag row: sets the uri to the eolas Memory URI.
//
// The script is idempotent: tag rows that already have a non-empty uri are
// skipped. Safe to re-run if interrupted.
//
// Refs: lucas42/lucos_media_metadata_api#239

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// eolasEntity represents one item from an eolas type_list response.
type eolasEntity struct {
	ID   int    `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// eolasClient wraps HTTP calls to the eolas API.
type eolasClient struct {
	origin string
	key    string
	http   *http.Client
}

func newEolasClient(origin, key string) *eolasClient {
	return &eolasClient{
		origin: strings.TrimSuffix(origin, "/"),
		key:    key,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *eolasClient) get(path string, out interface{}) error {
	req, err := http.NewRequest("GET", c.origin+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}

// post POSTs JSON to the eolas API. Returns the created entity on 201, or the
// existing entity on 409 (already_exists).
func (c *eolasClient) post(path string, payload interface{}) (*eolasEntity, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.origin+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var result eolasEntity
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w (body: %s)", err, string(body))
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return &result, nil
	case http.StatusConflict: // already_exists
		return &result, nil
	default:
		return nil, fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
}

// listMemories fetches all existing Memory entities from eolas.
// Returns a map keyed by lower-cased name for O(1) lookup.
func (c *eolasClient) listMemories() (map[string]*eolasEntity, error) {
	var items []eolasEntity
	if err := c.get("/metadata/memory/list/", &items); err != nil {
		return nil, fmt.Errorf("listing memories: %w", err)
	}
	byName := make(map[string]*eolasEntity, len(items))
	for i := range items {
		key := strings.ToLower(items[i].Name)
		if _, dup := byName[key]; !dup {
			byName[key] = &items[i]
		}
	}
	return byName, nil
}

// findOrCreate looks up a Memory entity by name; creates it if absent.
func (c *eolasClient) findOrCreate(name string, existing map[string]*eolasEntity) (*eolasEntity, error) {
	if e, ok := existing[strings.ToLower(name)]; ok {
		return e, nil
	}
	// Not found — create via the write API.
	entity, err := c.post("/api/metadata/memory/", map[string]interface{}{
		"name": name,
	})
	if err != nil {
		return nil, fmt.Errorf("creating memory %q: %w", name, err)
	}
	// Cache so subsequent calls for the same name resolve immediately.
	existing[strings.ToLower(name)] = entity
	return entity, nil
}

func main() {
	dbPath := flag.String("db", "", "Path to the SQLite database file (required)")
	eolasOrigin := flag.String("eolas-origin", "", "eolas base URL, e.g. https://eolas.l42.eu (required)")
	eolasKey := flag.String("eolas-key", "", "eolas API key (required)")
	dryRun := flag.Bool("dry-run", false, "Print what would be done without making any changes")
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "error: -db is required")
		flag.Usage()
		os.Exit(1)
	}
	if *eolasOrigin == "" {
		fmt.Fprintln(os.Stderr, "error: -eolas-origin is required")
		flag.Usage()
		os.Exit(1)
	}
	if *eolasKey == "" {
		fmt.Fprintln(os.Stderr, "error: -eolas-key is required")
		flag.Usage()
		os.Exit(1)
	}

	// Open database.
	db, err := sqlx.Open("sqlite3", *dbPath+"?_busy_timeout=10000")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.MustExec("PRAGMA journal_mode=WAL;")
	db.MustExec("PRAGMA foreign_keys = ON;")

	// Collect distinct freetext values (no uri set yet).
	var freeValues []string
	err = db.Select(&freeValues, `
		SELECT DISTINCT value FROM tag
		WHERE predicateid = 'memory' AND (uri IS NULL OR uri = '')
		ORDER BY value
	`)
	if err != nil {
		log.Fatalf("query freetext values: %v", err)
	}

	if len(freeValues) == 0 {
		fmt.Println("No freetext memory tags found; nothing to migrate.")
		return
	}

	fmt.Printf("Found %d distinct freetext memory value(s):\n", len(freeValues))
	for _, v := range freeValues {
		fmt.Printf("  %q\n", v)
	}

	if *dryRun {
		fmt.Println("\n[dry-run] No changes made.")
		return
	}

	// Connect to eolas.
	client := newEolasClient(*eolasOrigin, *eolasKey)

	// Fetch existing Memory entities.
	fmt.Println("\nFetching existing Memory entities from eolas...")
	existing, err := client.listMemories()
	if err != nil {
		log.Fatalf("fetch memories: %v", err)
	}
	fmt.Printf("Found %d existing Memory entity/entities.\n", len(existing))

	// Resolve URIs for all distinct values.
	valueToURI := make(map[string]string, len(freeValues))
	for _, name := range freeValues {
		entity, err := client.findOrCreate(name, existing)
		if err != nil {
			log.Fatalf("resolve %q: %v", name, err)
		}
		valueToURI[name] = entity.URI
		fmt.Printf("  %q → %s\n", name, entity.URI)
	}

	// Update tag rows in a single transaction.
	tx, err := db.Beginx()
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}

	totalUpdated := int64(0)
	for name, uri := range valueToURI {
		res, err := tx.Exec(`
			UPDATE tag SET uri = ?
			WHERE predicateid = 'memory'
			  AND value = ?
			  AND (uri IS NULL OR uri = '')
		`, uri, name)
		if err != nil {
			_ = tx.Rollback()
			log.Fatalf("update tags for %q: %v", name, err)
		}
		rows, _ := res.RowsAffected()
		totalUpdated += rows
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("\nMigration complete. %d tag row(s) updated across %d distinct value(s).\n",
		totalUpdated, len(freeValues))
}
