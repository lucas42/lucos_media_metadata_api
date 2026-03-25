# ADR 001: Multi-Value Fields Architecture

**Date:** 2026-03-24
**Status:** Accepted
**Author:** lucos-architect[bot]
**Discussion:** [GitHub Issue #34](https://github.com/lucas42/lucos_media_metadata_api/issues/34)

## Context

The database currently enforces a `UNIQUE(trackid, predicateid)` constraint on the `tag` table, meaning each predicate can appear at most once per track. All API logic — from `getAllTagsForTrack` returning `map[string]string` to the `Track` struct's `Tags` field — relies on this one-value-per-predicate assumption.

In reality, several predicates are inherently multi-valued: a track can have multiple languages, composers, producers, and so on. The current workaround is comma-separated values stored as a single string (`"en,fr"`), with splitting logic scattered across consumers:

- `rdfgen/rdf.go` uses `splitCSV()` in `mapPredicate()` for composer, producer, language, offence, about, and mentions
- `lucos_media_metadata_manager` uses `formfields.php` to identify delimiter-based fields
- Search via `searchByPredicates` does exact string matching (`tag.value = ?`), so searching for `p.language=fr` will not match a track stored as `"en,fr"`

This ADR documents the design decisions for native multi-value field support, agreed through the discussion in issue #34.

## Decision

### 1. Introduce a v3 API, not backwards-compatible v2 changes

The v2 API returns tags as `map[string]string`:

```json
{"tags": {"artist": "Bob Dylan", "language": "en,fr"}}
```

Multi-value support changes the type of some tag values from `string` to `[]string`, which is a breaking change for every consumer. Rather than attempting backwards compatibility, we introduce v3 endpoints at `/v3/tracks` (and related paths).

This follows the established deprecation pattern: v1 endpoints already return `410 Gone` via `V1GoneController`, proving the approach works. v2 endpoints will continue to operate with comma-separated values during the transition, then follow the same deprecation path.

**Trade-off:** Running two API versions concurrently adds maintenance overhead during the transition period. This is accepted because it allows consumers to migrate independently on their own timelines, rather than requiring a coordinated big-bang cutover.

### 2. Mixed types in v3: strings for single-value, arrays for multi-value

In v3, tag values are typed according to whether the predicate is multi-valued:

```json
{
  "tags": {
    "artist": "Bob Dylan",
    "title": "Blowin' in the Wind",
    "language": ["en", "fr"],
    "composer": ["Bob Dylan"]
  }
}
```

Single-value predicates remain strings. Multi-value predicates are always arrays, even when they currently have only one value.

The alternative considered was making all tag values arrays for type uniformity. This was rejected because consumers use specific tags for specific purposes — they do `track.Tags["artist"]` and expect a string. Wrapping every single-value field in an array forces `track.Tags["artist"][0]` everywhere, which adds ceremony without solving a real problem.

**Trade-off:** The `tags` map becomes `map[string]interface{}` internally (rather than `map[string]string`), which is less pleasant to work with in Go. The `DecodeTrack` function will need a custom JSON unmarshaller that validates types against the multi-value predicate set. This is where bugs are most likely to hide, requiring thorough test coverage.

### 3. GET and PUT/PATCH use the same shape

If GET returns `"language": ["en", "fr"]`, then PUT/PATCH must accept exactly that shape too. No asymmetry between read and write formats.

For multi-value predicates, PUT/PATCH replaces all values with the provided array:
- `"language": ["fr"]` sets language to exactly `["fr"]` (removes any other values)
- `"language": []` clears all language values

There is no "append a single value" semantic in the tracks endpoint. If fine-grained add/remove of individual values within a multi-value predicate is needed in future, it could be handled by a dedicated `/tags` endpoint. (Note: v1 had a `/tags` endpoint that no consumers used, so this is deferred unless there is demonstrated need.)

### 4. Multi-value predicate list lives in code, not the database

The set of predicates that support multiple values is defined as a Go constant in the API codebase. The example below is based on the predicates currently using `splitCSV()` in `rdfgen/rdf.go`; the exact set is not yet finalised and will be agreed in #37 with owner input:

```go
// Illustrative — the definitive set will be agreed in #37.
var multiValuePredicates = map[string]bool{
    "composer":  true,
    "producer":  true,
    "language":  true,
    "offence":   true,
    "about":     true,
    "mentions":  true,
}
```

The alternative considered was a `predicate_schema` table in the database. This was rejected because:
- The multi-value designation is a design-time decision, not runtime configuration
- A change to this list affects the API's wire format — that is a breaking change that should be the purview of code and deployment, not database state
- The codebase already has a `predicate` table for predicate existence, but adding schema semantics to it conflates storage with API behaviour

The `formfields.php` metadata in `lucos_media_metadata_manager` and this predicate list in the API serve different purposes (UI rendering vs. storage/serialisation). Some duplication between them is acceptable. When the list needs extending, it requires code changes in both repos — this is a feature, as it forces deliberate consideration.

### 5. Database migration: drop UNIQUE constraint, split CSV values

The `UNIQUE(trackid, predicateid)` constraint on the `tag` table must be removed to allow multiple rows with the same track+predicate pair. Existing comma-separated values for multi-value predicates must be split into separate rows.

The migration approach:
1. Build a read-only audit script that identifies all tags currently using comma separation and validates that splitting them produces correct results (handling edge cases like values that legitimately contain commas)
2. Create a new `tag` table without the UNIQUE constraint (SQLite requires table recreation for constraint removal)
3. Copy data, splitting comma-separated values for predicates in the multi-value set into individual rows
4. Swap tables

The `REPLACE INTO` statement in `updateTag` (which relies on the UNIQUE constraint for upsert behaviour) must be replaced with explicit INSERT/UPDATE logic before or during this migration.

### 6. Internal refactor before v3: `map[string]string` to `[]Tag`

Before implementing v3 endpoints, the internal representation should be decoupled from the wire format. `getAllTagsForTrack` currently returns `map[string]string` — this should be refactored to return `[]Tag` (using the existing `Tag` struct), while v2 serialisation continues to produce `map[string]string`.

This refactoring is a code-only change with no schema or API impact, and it makes the v3 implementation cleaner: v3 serialisation can group `[]Tag` entries by predicate, producing arrays for multi-value predicates and strings for single-value ones.

## Consequences

### Positive

- **Search works correctly.** With each value in its own row, `searchByPredicates` (`INNER JOIN tag ... AND tag.value = ?`) matches tracks that have a value as *any* of their multi-value entries — no more missed results. This is arguably the biggest single win.
- **RDF export simplifies.** The `splitCSV()` calls in `rdfgen/rdf.go` can be removed, since each value is already its own database row. The hardcoded knowledge of which predicates are multi-valued moves to a single authoritative location.
- **Clear consumer migration path.** v2 continues working unchanged; consumers migrate to v3 independently. The v1→v2 deprecation provides a proven pattern.
- **Schema is explicit.** The multi-value predicate set is a visible, version-controlled constant rather than an implicit convention scattered across consumers.

### Negative

- **Two concurrent API versions during transition.** Both v2 and v3 must be maintained, tested, and documented until all consumers have migrated and v2 is removed.
- **Mixed types in Go.** `map[string]interface{}` is less ergonomic than `map[string]string`. Custom JSON marshalling/unmarshalling adds complexity.
- **Migration risk.** Splitting comma-separated values is straightforward in the common case but has edge cases (values containing commas, inconsistent delimiters, whitespace handling). The audit-first approach mitigates this but does not eliminate it.
- **Duplication of predicate knowledge.** The multi-value list exists in the API codebase and in `formfields.php` in the manager. These must be kept in sync manually.

## Known Consumers

The following systems read from or write to this API and will need migration plans:

| Consumer | Language | Reads | Writes | Notes |
|---|---|---|---|---|
| lucos_media_metadata_manager | PHP | Yes | Yes | Primary UI for tag editing |
| lucos_media_manager | Java | Yes | No | Reads tags for playback weighting |
| lucos_arachne ingestor | Python | Yes | No | Reads RDF export |
| lucos_media_import | — | Yes | Yes | Bulk import |
| lucos_media_weightings | — | Yes | Yes | Reads metadata, writes to `/v2/tracks/{id}/weighting` |

## Implementation Order

1. Audit all tag consumers and comma-separated field usage (#35)
2. Internal refactor: `getAllTagsForTrack` returns `[]Tag`, v2 wire format unchanged (#36)
3. Define `multiValuePredicates` constant in Go code (#37 — needs owner input on exact predicate list)
4. Database migration: drop UNIQUE constraint, split CSV values (#38)
5. Implement v3 endpoints with mixed string/array tag values (#39)
6. **Board approval gate** — V3 API review before consumer migration
7. Update rdfgen to remove `splitCSV` calls (#40)
8. Migrate consumers from v2 to v3 (#41)
9. Deprecate and remove v2 endpoints (#42)
