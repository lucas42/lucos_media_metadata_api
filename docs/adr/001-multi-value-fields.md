# ADR 001: Multi-Value Fields Architecture

**Date:** 2026-03-24
**Status:** Accepted
**Author:** lucos-architect[bot]
**Discussion:** [GitHub Issue #34](https://github.com/lucas42/lucos_media_metadata_api/issues/34), [GitHub Issue #45](https://github.com/lucas42/lucos_media_metadata_api/issues/45)

## Context

The database currently enforces a `UNIQUE(trackid, predicateid)` constraint on the `tag` table, meaning each predicate can appear at most once per track. All API logic — from `getAllTagsForTrack` returning `map[string]string` to the `Track` struct's `Tags` field — relies on this one-value-per-predicate assumption.

In reality, several predicates are inherently multi-valued: a track can have multiple languages, composers, producers, and so on. The current workaround is comma-separated values stored as a single string (`"en,fr"`), with splitting logic scattered across consumers:

- `rdfgen/rdf.go` uses `splitCSV()` in `mapPredicate()` for composer, producer, language, offence, about, and mentions
- `lucos_media_metadata_manager` uses `formfields.php` to identify delimiter-based fields
- Search via `searchByPredicates` does exact string matching (`tag.value = ?`), so searching for `p.language=fr` will not match a track stored as `"en,fr"`

This ADR documents the design decisions for native multi-value field support and the broader v3 API changes, agreed through the discussions in issues #34 and #45.

## Decision

### 1. Introduce a v3 API, not backwards-compatible v2 changes

The v2 API returns tags as `map[string]string`:

```json
{"tags": {"artist": "Bob Dylan", "language": "en,fr"}}
```

Multi-value support fundamentally changes the tag representation, which is a breaking change for every consumer. Rather than attempting backwards compatibility, we introduce v3 endpoints at `/v3/tracks` (and related paths).

This follows the established deprecation pattern: v1 endpoints already return `410 Gone` via `V1GoneController`, proving the approach works. v2 endpoints will continue to operate with comma-separated values during the transition, then follow the same deprecation path.

**Trade-off:** Running two API versions concurrently adds maintenance overhead during the transition period. This is accepted because it allows consumers to migrate independently on their own timelines, rather than requiring a coordinated big-bang cutover.

### 2. Structured tag values in v3: objects with `name` and optional `uri`

The v3 tag representation uses structured objects rather than plain strings. Each predicate maps to an array of value objects. This design was agreed in #45 to support both the immediate multi-value requirement and future controlled vocabulary migrations (where tag values link to entities in lucos_eolas).

**v3 response format — tags as a map of predicate to value arrays:**

```json
{
  "tags": {
    "artist": [
      {"name": "Bob Dylan"}
    ],
    "language": [
      {"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"},
      {"name": "French", "uri": "https://eolas.l42.eu/metadata/language/fr/"}
    ],
    "composer": [
      {"name": "Bob Dylan"}
    ],
    "singalong": [
      {"name": "chorus only"}
    ]
  }
}
```

Key properties:
- **All predicates use the same shape.** Every tag value is an array of objects, whether single-value or multi-value. This was a revision from an earlier proposal (in #34) to use mixed types (strings for single-value, arrays for multi-value), which was rejected in #45 because the structured object format makes all values uniform.
- **`name` is the human-readable string.** The field is called `name` (not `value` or `label`) for consistency with lucos_eolas naming conventions.
- **`uri` is optional.** Present when the tag value references a controlled vocabulary entity in lucos_eolas or another system. Absent for freetext values.
- **Both `name` and `uri` are stored in the database.** Synchronous lookups to lucos_eolas on every API response would be a performance non-starter. The denormalised `name` is kept in sync via a loganne webhook for `itemUpdated` events plus a periodic reconciliation job — the standard lucos belt-and-braces pattern for denormalised data.

The earlier proposal (from #34) of mixed string/array types — where single-value predicates stayed as strings and multi-value predicates became arrays — was superseded by this design. The structured format was chosen because it enables post-v3 controlled vocabulary migrations as data-only changes (adding `uri` fields) rather than further API version bumps.

**Trade-off:** The response is more verbose than the v2 `map[string]string`. Every tag value is now an object in an array, even for simple freetext single-value predicates. This is accepted because it provides a uniform, future-proof format that avoids mixed types and supports the planned vocabulary migrations.

### 3. GET and PUT/PATCH use the same shape

If GET returns a structured tag format, then PUT/PATCH must accept exactly that shape too. No asymmetry between read and write formats.

For multi-value predicates, PUT/PATCH replaces all values with the provided array:
- `"language": [{"name": "French", "uri": "..."}]` sets language to exactly that (removes any other values)
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

### 5. Database migration: drop UNIQUE constraint, split CSV values

The `UNIQUE(trackid, predicateid)` constraint on the `tag` table must be removed to allow multiple rows with the same track+predicate pair. Existing comma-separated values for multi-value predicates must be split into separate rows.

The migration approach:
1. Build a read-only audit script that identifies all tags currently using comma separation and validates that splitting them produces correct results (handling edge cases like values that legitimately contain commas)
2. Create a new `tag` table without the UNIQUE constraint (SQLite requires table recreation for constraint removal)
3. Copy data, splitting comma-separated values for predicates in the multi-value set into individual rows
4. Swap tables

The `REPLACE INTO` statement in `updateTag` (which relies on the UNIQUE constraint for upsert behaviour) must be replaced with explicit INSERT/UPDATE logic before or during this migration.

The tag table will also need a `uri` column to support the structured value format. This can be added as part of the table recreation.

### 6. Internal refactor before v3: `map[string]string` to `[]Tag`

Before implementing v3 endpoints, the internal representation should be decoupled from the wire format. `getAllTagsForTrack` currently returns `map[string]string` — this should be refactored to return `[]Tag` (using the existing `Tag` struct), while v2 serialisation continues to produce `map[string]string`.

This refactoring is a code-only change with no schema or API impact, and it makes the v3 implementation cleaner: v3 serialisation can group `[]Tag` entries by predicate, producing the structured value objects.

### 7. Additional v3 changes (bundled from #45)

Since v3 is already a breaking change, several other improvements are bundled to avoid multiple migration rounds for consumers:

- **Rename `trackid` to `id` in JSON output.** The `track` prefix is redundant when the resource is already accessed via `/v3/tracks/{id}`. Every other resource (e.g. collections) uses unprefixed identifiers.
- **Remove debug weighting fields from the default response.** `_random_weighting` and `_cum_weighting` are internal implementation details that should not appear in the API response.
- **Structured error responses.** v2 returns plain-text errors with status codes; v3 returns JSON errors (e.g. `{"error": "Track Not Found", "code": "not_found"}`).
- **Richer pagination response.** Add `page` and `totalTracks` fields alongside the existing `totalPages`.

The weighting endpoint (`/v3/tracks/{id}/weighting`) deliberately remains plain-text, as its simplicity is a feature — it has a single consumer and returns a single number.

### 8. Deletion handling for orphaned tags

When a lucos_eolas entity referenced by a tag is deleted, the recommended approach is **Option A: keep the orphan, clear the URI**. Set the `uri` field to null but keep the `name`. The tag effectively reverts to a freetext value.

This is preferred because:
- Entity deletion is rare and usually part of a merge (where references should be updated before deletion)
- Silently deleting tags is disproportionate — a track composed by "Bach" is still composed by "Bach" even if the Bach entity is removed from lucos_eolas
- Keeping the `name` preserves data for humans while removing the broken link

Note: this recommendation was signed off in the #45 discussion.

## Consequences

### Positive

- **Search works correctly.** With each value in its own row, `searchByPredicates` (`INNER JOIN tag ... AND tag.value = ?`) matches tracks that have a value as *any* of their multi-value entries — no more missed results. This is arguably the biggest single win.
- **RDF export simplifies.** The `splitCSV()` calls in `rdfgen/rdf.go` can be removed, since each value is already its own database row. The hardcoded knowledge of which predicates are multi-valued moves to a single authoritative location.
- **Clear consumer migration path.** v2 continues working unchanged; consumers migrate to v3 independently. The v1→v2 deprecation provides a proven pattern.
- **Future-proof tag format.** The structured `name`/`uri` format means post-v3 controlled vocabulary migrations (moving predicates to lucos_eolas) are data-only changes — no further API version bumps needed.
- **Schema is explicit.** The multi-value predicate set is a visible, version-controlled constant rather than an implicit convention scattered across consumers.

### Negative

- **Two concurrent API versions during transition.** Both v2 and v3 must be maintained, tested, and documented until all consumers have migrated and v2 is removed.
- **More complex internal types.** The structured tag format requires more sophisticated JSON marshalling/unmarshalling than the current `map[string]string`. Custom marshalling logic is where bugs are most likely to hide.
- **Data sync overhead.** Denormalising `name` from lucos_eolas requires the loganne webhook and periodic reconciliation mechanisms — additional infrastructure to build and maintain.
- **Migration risk.** Splitting comma-separated values is straightforward in the common case but has edge cases (values containing commas, inconsistent delimiters, whitespace handling). The audit-first approach mitigates this but does not eliminate it.

## Known Consumers

The following systems read from or write to this API and will need migration plans:

| Consumer | Language | Reads | Writes | Notes |
|---|---|---|---|---|
| lucos_media_metadata_manager | PHP | Yes | Yes | Primary UI for tag editing |
| lucos_media_manager | Java | Yes | No | Reads tags for playback weighting |
| lucos_arachne ingestor | Python | Yes | No | Reads RDF export |
| lucos_media_import | Python | No | Yes | Bulk import |
| lucos_media_weightings | Python | Yes | Yes | Reads metadata, writes to `/v2/tracks/{id}/weighting` |

## Implementation Order

1. Audit all tag consumers and comma-separated field usage (#35)
2. Internal refactor: `getAllTagsForTrack` returns `[]Tag`, v2 wire format unchanged (#36)
3. Define `multiValuePredicates` constant in Go code (#37 — needs owner input on exact predicate list)
4. Database migration: drop UNIQUE constraint, split CSV values, add `uri` column (#38)
5. Implement v3 endpoints with structured tag values and bundled changes (#39)
6. **Board approval gate** — V3 API review before consumer migration
7. Update rdfgen to remove `splitCSV` calls (#40)
8. Migrate consumers from v2 to v3 (#41)
9. Deprecate and remove v2 endpoints (#42)

## §9 Tag Value Validation

v3 write endpoints (`PUT /v3/tracks/...`, `PATCH /v3/tracks/...`) validate tag values at the wire boundary and return `400 invalid_tag_value` for invalid inputs. The `predicate` field in the error response identifies the offending predicate.

| Request shape | Server behaviour |
|---|---|
| `tags` key omitted | Do not touch any tags |
| `tags: {}` | Do not touch any tags |
| `tags: {"comment": [{"name": "X"}]}` | Replace `comment` with `[X]` |
| `tags: {"comment": []}` | Clear all values for `comment` |
| `tags: {"comment": null}` | **400** `invalid_tag_value` — use `[]` to clear |
| `tags: {"comment": [{"name": ""}]}` | **400** `invalid_tag_value` — name must be non-empty |
| `tags: {"comment": [{"name": "", "uri": ""}]}` | **400** `invalid_tag_value` — name must be non-empty |
| `tags: {"comment": [{"name": "X", "uri": null}]}` | Replace `comment` with `[X]`, no URI bound |
| `tags: {"comment": [{"name": "X"}]}` (uri omitted) | Equivalent to uri null |
| `tags: {"album": [{"uri": "/albums/1"}]}` (name omitted) | Valid — URI-only; server resolves name |

The most important distinction: **omission (`tags` absent or `{}`) is "leave alone"**, while **`[]` is "clear"**.
