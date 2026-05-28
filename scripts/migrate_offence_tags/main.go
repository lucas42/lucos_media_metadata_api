// migrate_offence_tags converts existing freetext offence tag values to Offence
// entity references in lucos_eolas.
//
// The media metadata DB stores offence tags as slugs (e.g. "domestic-abuse"),
// while lucos_eolas stores Offence entities by their display name
// (e.g. "Domestic Abuse"). This script bridges that gap using the canonical
// slug→name mapping from lucos_media_metadata_manager's formfields.php.
//
// Run this script as part of the coordinated release window:
//
//  1. Ask users to pause edits to offence tags.
//  2. Run the migration via Docker (the binary is included in the API image):
//       docker run --rm \
//         -v lucos_media_metadata_api_db:/var/lib/media-metadata/ \
//         lucas42/lucos_media_metadata_api \
//         migrate_offence_tags \
//         -db /var/lib/media-metadata/media.sqlite \
//         -eolas-origin https://eolas.l42.eu \
//         -eolas-key <KEY_LUCOS_EOLAS value>
//  3. Deploy the updated API (formfields.php switched to type:search) together
//     with the updated lucos_media_metadata_manager so the cutover is atomic.
//  4. Resume edits.
//
// What the script does:
//   - Reads all distinct freetext offence tag values (rows where uri is empty).
//   - For each distinct value, translates the slug to a canonical name using the
//     slugToName table below (sourced from formfields.php). Then looks up the
//     matching Offence entity in eolas by that name.
//   - Unmatched slugs (values not in the table): warns loudly and creates a new
//     Offence entity in eolas using the raw slug as the name. This should not
//     happen in practice — formfields.php has been a controlled select.
//   - Rewrites each tag row: sets the uri to the eolas Offence URI.
//
// The script is idempotent: tag rows that already have a non-empty uri are
// skipped. Safe to re-run if interrupted.

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

// slugToName maps the formfields.php slug keys (stored in the DB as tag values)
// to the canonical display names used by the lucos_eolas Offence entities.
// Sourced from lucos_media_metadata_manager/src/formfields.php, "offence" block.
var slugToName = map[string]string{
	"swearing":                 "Swearing",
	"slurs":                    "Slurs",
	"sacrilege":                "Sacrilege",
	"violence":                 "Violence",
	"war":                      "War",
	"lèse-majesté":             "Lèse-majesté",
	"jingoism":                 "Jingoism",
	"smut":                     "Smut",
	"alcohol":                  "Alcohol",
	"drugs":                    "Drugs",
	"kink":                     "Kink",
	"arson":                    "Arson",
	"domestic-abuse":           "Domestic Abuse",
	"colonialism":              "Colonialism",
	"sexual-assault":           "Sexual Assault",
	"sex-work":                 "Sex Work",
	"animal-cruelty":           "Animal Cruelty",
	"fascism":                  "Fascism",
	"self-harm":                "Self Harm (including suicide)",
	"gambling":                 "Gambling",
	"racism":                   "Racism",
	"religious-discrimination": "Religious Discrimination",
	"sexism":                   "Sexism",
	"ableism":                  "Ableism",
	"homophobia":               "Homophobia",
	"transphobia":              "Transphobia",
}

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

// listOffences fetches all existing Offence entities from eolas.
// Returns a map keyed by lower-cased display name for O(1) lookup.
func (c *eolasClient) listOffences() (map[string]*eolasEntity, error) {
	var items []eolasEntity
	if err := c.get("/metadata/offence/list/", &items); err != nil {
		return nil, fmt.Errorf("listing offences: %w", err)
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

// resolveSlug translates a DB slug value to a canonical display name using the
// slugToName table, then looks up the matching eolas entity.
// If the slug is not in the table, falls through to a direct name lookup
// (handling any legacy values stored as display names) and creates a new entity
// if still not found.
func (c *eolasClient) resolveSlug(slug string, existing map[string]*eolasEntity) (*eolasEntity, error) {
	// Translate slug → canonical name.
	canonicalName, inTable := slugToName[slug]
	if !inTable {
		// Unknown slug — not in the formfields.php controlled vocab.
		// Try a direct case-insensitive lookup in case someone entered a display name.
		if e, ok := existing[strings.ToLower(slug)]; ok {
			log.Printf("WARNING: slug %q not in slugToName table but matched eolas entity %q by name — possible legacy display-name value", slug, e.Name)
			return e, nil
		}
		// Still not found — create a new entity and warn loudly.
		log.Printf("WARNING: slug %q not in slugToName table and no matching eolas entity — creating new Offence entity", slug)
		entity, err := c.post("/api/metadata/offence/", map[string]interface{}{
			"name": slug,
		})
		if err != nil {
			return nil, fmt.Errorf("creating offence for unknown slug %q: %w", slug, err)
		}
		existing[strings.ToLower(slug)] = entity
		return entity, nil
	}

	// Look up the canonical name in the existing eolas entities.
	if e, ok := existing[strings.ToLower(canonicalName)]; ok {
		return e, nil
	}

	// Entity not in eolas — shouldn't happen since eolas#263 pre-populated the
	// full vocab, but create it rather than failing the migration.
	log.Printf("WARNING: slug %q maps to name %q but no matching eolas entity found — creating", slug, canonicalName)
	entity, err := c.post("/api/metadata/offence/", map[string]interface{}{
		"name": canonicalName,
	})
	if err != nil {
		return nil, fmt.Errorf("creating offence %q (from slug %q): %w", canonicalName, slug, err)
	}
	existing[strings.ToLower(canonicalName)] = entity
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

	// Collect distinct freetext slug values (no uri set yet).
	var freeSlugs []string
	err = db.Select(&freeSlugs, `
		SELECT DISTINCT value FROM tag
		WHERE predicateid = 'offence' AND (uri IS NULL OR uri = '')
		ORDER BY value
	`)
	if err != nil {
		log.Fatalf("query freetext values: %v", err)
	}

	if len(freeSlugs) == 0 {
		fmt.Println("No freetext offence tags found; nothing to migrate.")
		return
	}

	fmt.Printf("Found %d distinct freetext offence slug(s):\n", len(freeSlugs))
	for _, v := range freeSlugs {
		name, ok := slugToName[v]
		if ok {
			fmt.Printf("  %q → %q\n", v, name)
		} else {
			fmt.Printf("  %q → (unknown slug — will warn at migration time)\n", v)
		}
	}

	if *dryRun {
		fmt.Println("\n[dry-run] No changes made.")
		return
	}

	// Connect to eolas.
	client := newEolasClient(*eolasOrigin, *eolasKey)

	// Fetch existing Offence entities.
	fmt.Println("\nFetching existing Offence entities from eolas...")
	existing, err := client.listOffences()
	if err != nil {
		log.Fatalf("fetch offences: %v", err)
	}
	fmt.Printf("Found %d existing Offence entity/entities.\n", len(existing))

	// Resolve each slug to an eolas entity URI.
	slugToURI := make(map[string]string, len(freeSlugs))
	for _, slug := range freeSlugs {
		entity, err := client.resolveSlug(slug, existing)
		if err != nil {
			log.Fatalf("resolve slug %q: %v", slug, err)
		}
		slugToURI[slug] = entity.URI
		fmt.Printf("  %q → %s\n", slug, entity.URI)
	}

	// Update tag rows in a single transaction.
	tx, err := db.Beginx()
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}

	totalUpdated := int64(0)
	for slug, uri := range slugToURI {
		res, err := tx.Exec(`
			UPDATE tag SET uri = ?
			WHERE predicateid = 'offence'
			  AND value = ?
			  AND (uri IS NULL OR uri = '')
		`, uri, slug)
		if err != nil {
			_ = tx.Rollback()
			log.Fatalf("update tags for slug %q: %v", slug, err)
		}
		rows, _ := res.RowsAffected()
		totalUpdated += rows
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("\nMigration complete. %d tag row(s) updated across %d distinct slug(s).\n",
		totalUpdated, len(freeSlugs))
}
