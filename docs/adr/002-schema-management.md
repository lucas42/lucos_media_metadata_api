# ADR 002: Schema Management

**Date:** 2026-05-30
**Status:** Accepted
**Author:** lucos-architect[bot]
**Discussion:** [GitHub Issue #291](https://github.com/lucas42/lucos_media_metadata_api/issues/291) (approach approved by lucas42 via +1 on the [Option 2 recommendation](https://github.com/lucas42/lucos_media_metadata_api/issues/291#issuecomment-4583505150)); see also [#290](https://github.com/lucas42/lucos_media_metadata_api/issues/290) (removal of the one-off migration machinery).

## Context

`lucos_media_metadata_api` has no formal schema-management strategy. All DDL lives in `DBInit()` (`api/database.go`): it creates tables if absent, then runs a growing list of idempotent, content-driven `ALTER`/backfill steps on **every** startup — `ColExists("collection", "icon")`, `needsArtistTagMigration()`, `needsEolasDataMigration()`, and so on. There is no `schema_migrations` version table.

The defect in this model is not "no framework" — it is that **schema evolution is encoded as conditionals re-evaluated on every boot, with the data shape acting as an implicit version number.** Each past change leaves a permanent `if` that runs forever, and detecting "has this change already been applied?" requires inspecting table/column state rather than reading a recorded version. The list only grows, and the next genuine schema change has nowhere to live except another such conditional.

[#290](https://github.com/lucas42/lucos_media_metadata_api/issues/290) removes the completed one-off data migrations and rebases `DBInit()` onto the current column shapes. That leaves the forward question this ADR answers: **how should the next genuine schema change be handled?**

### Constraints

- **SQLite, single production instance, WAL mode.** Migrations run once, at startup, in the single API process before it serves traffic — there is no concurrent-runner race, so no advisory locking is needed.
- **A blank environment must remain trivial to spin up from scratch.**
- **No need to recover data from older shapes.** We have explicitly agreed (in #290) that backward data migration is not a requirement — which means **down-migrations are out of scope**.

## Decision

Adopt **Option 2: thin versioned migrations, no external dependency.**

### 1. A `schema_migrations` version table plus ordered, embedded `.sql` files

Schema evolution becomes a set of forward-only SQL files, embedded into the binary with `embed.FS`, applied in lexical order, each recorded in a `schema_migrations` table once applied. The runner is roughly 50 lines of Go and adds no dependency:

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

// applyMigrations runs all not-yet-applied migration files in lexical order,
// each atomically applied-and-recorded in a single transaction. Any failure
// is fatal: a half-migrated database must not start serving traffic.
func (store Datastore) applyMigrations() {
    store.DB.MustExec(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version    TEXT PRIMARY KEY,
        applied_at TEXT NOT NULL DEFAULT (datetime('now'))
    )`)
    files, err := fs.Glob(migrationsFS, "migrations/*.sql")
    if err != nil { panic(err) }
    sort.Strings(files)
    for _, f := range files {
        version := filepath.Base(f)
        // SELECT EXISTS always returns exactly one row (0 or 1) — no sql.ErrNoRows
        // to special-case, and no driver-dependent integer-to-bool coercion.
        var exists int
        if err := store.DB.Get(&exists, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", version); err != nil {
            panic(err)
        }
        if exists == 1 { continue }
        sqlText, err := migrationsFS.ReadFile(f)
        if err != nil { panic(err) }
        slog.Info("Applying migration", slog.String("version", version))
        tx := store.DB.MustBegin()
        tx.MustExec(string(sqlText))
        tx.MustExec("INSERT INTO schema_migrations (version) VALUES (?)", version)
        if err := tx.Commit(); err != nil { panic(err) } // fail loud; do not start half-migrated
    }
}
```

The key property this restores, which the current model lacks: **a change is expressed once, applied once, recorded, and never re-evaluated.** The version table is the single source of truth for "what shape is this database at" — replacing the content-driven detection scattered through `DBInit()`.

### 2. Baseline expressed as `migrations/0001_baseline.sql`, using `CREATE TABLE IF NOT EXISTS`

The current schema becomes the first migration, `migrations/0001_baseline.sql`, containing the post-#290 `CREATE TABLE IF NOT EXISTS` statements for every table. The `IF NOT EXISTS` clause is what makes the rollout safe **with no manual stamping step**:

- **Blank environment:** no tables, no `schema_migrations` → the baseline runs, creates every table, and records `0001`. (Satisfies the "trivial from scratch" constraint.)
- **Existing production database** (already at the current shape after #290, no `schema_migrations`): the baseline's `CREATE TABLE IF NOT EXISTS` statements are no-ops against the existing tables, and `0001` is then recorded as applied. The existing data is untouched; the database is simply *adopted at baseline*.

This is the decision that unblocks [#290](https://github.com/lucas42/lucos_media_metadata_api/issues/290)'s final step: **#290 lands the baseline as `migrations/0001_baseline.sql`, not as inline `CREATE TABLE` in Go.** There is no special-cased "version 0" — a fresh database and an existing one converge through the same mechanism.

Only the baseline migration needs to be idempotent in this way. Every subsequent migration (`0002+`) is plain forward DDL gated solely by the version record, and must **not** be written defensively (no `IF NOT EXISTS`, no `ColExists`-style guards) — re-running is prevented by the version table, which is the entire point.

### 3. Migrations are forward-only

We do not write down-migrations. Given the constraints (single instance, no requirement to recover old data shapes), `down` files would be dead weight that is rarely correct and never exercised. If a released migration turns out to be wrong, the remedy is a **new corrective migration** with the next number — the same way every other forward-only system handles it. This is a deliberate scope reduction, and it is the main reason a framework (Option 3) is unnecessary here.

### 4. Conventions for writing migrations

- **Naming:** `NNNN_short_description.sql`, four-digit zero-padded monotonic prefix (`0001_baseline.sql`, `0002_add_track_isrc.sql`). Four digits is ample headroom; lexical sort gives apply order.
- **One logical change per file.**
- **Applied migrations are immutable.** Never edit a migration that has shipped — its hash of intent is "already recorded as applied" on every existing database, so an edit would never re-run and would cause drift. Correct or extend via a new migration.
- **Transactions and foreign keys (SQLite gotcha to respect):** the runner wraps each migration's execution *and* its `schema_migrations` insert in one transaction, so "applied" and "recorded" are atomic. A migration that needs to rebuild a table (SQLite still requires table recreation to change most constraints) must **not** use `PRAGMA foreign_keys = OFF` — that pragma is silently ignored inside a transaction. Use `PRAGMA defer_foreign_keys = ON` instead, which *is* honoured inside a transaction and defers foreign-key enforcement to `COMMIT`. (The legacy `migrateTagTableDropUnique` toggled `foreign_keys` outside a transaction; under the runner, table-rebuild migrations switch to `defer_foreign_keys`.)

### Options considered and rejected

- **Option 1 — collapse to create-only, no version tracking.** Simplest only because it does nothing: it leaves the next schema change with nowhere to go but back into the ad-hoc `DBInit()` blob we are removing, or a manual, unversioned `ALTER` on the production box. It does not answer the question the issue asks. Rejected — though it is the honest fallback if one believes schema changes will be vanishingly rare.
- **Option 3 — adopt a framework (goose / golang-migrate).** A framework earns its dependency by solving problems you have. The parts that justify one are exactly the parts we have ruled out: down-migrations (we do not recover old shapes), multiple drivers (SQLite only), multiple instances (single instance). That leaves "numbered files + a version table" — which *is* Option 2 — wrapped in a dependency that adds supply-chain surface and version churn for machinery we will not use. Disproportionate to a single, rebuildable SQLite database.

## Consequences

### Positive

- **Apply-once, never re-evaluated.** The startup path stops re-checking the shape of every past change on every boot. The version table is authoritative; the content-driven conditionals in `DBInit()` go away.
- **One mechanism, blank and existing converge.** A fresh database and the production database reach the same state through the same ordered files — no special baseline handling, no manual stamping.
- **No new dependency.** ~50 lines of well-understood Go and `embed.FS` (standard library). No framework to track, update, or absorb dependabot noise from.
- **Trivial from scratch, reproducible.** Spinning up a blank environment is "run the migrations" — deterministic and ordered.
- **A defined forward path.** The next schema change is one new `NNNN_*.sql` file. The "how do we handle the next change?" question now has a boring, repeatable answer.
- **Replicable across the estate.** The pattern is small enough to copy into the other single-instance Go + SQLite services without imposing a framework dependency on any of them, should a consistent approach be wanted later. (Not a commitment here — noted as a forward option.)

### Negative

- **We own the runner and the discipline.** ~50 lines of migration runner is ours to maintain, and every schema change now requires authoring a migration file. This is a small standing commitment, not zero — accepted as proportionate.
- **Forward-only means corrections cost a migration.** A bad migration cannot be rolled back; it is fixed by a new one. Acceptable given we do not need old-shape recovery, but it demands care that each migration is correct before release.
- **Startup-coupled.** Migrations run at process start and a failure is fatal by design (better than serving on a half-migrated schema). On a single WAL instance this is the correct trade, but it does mean a broken migration blocks the deploy at boot — which is the intended fail-loud behaviour, surfaced by the container healthcheck.

## Relationship to #290

This ADR is the design decision; the work splits as:

1. **#290** removes the one-off migration machinery and lands the baseline as `migrations/0001_baseline.sql` (per §2 above).
2. **The migration runner** (`applyMigrations`, the `schema_migrations` table, wiring into `DBInit()`/startup) is implemented per §1 and §4. This can be a small follow-up to, or part of, #290's baseline step — it does not require its own design round now that this ADR is settled.
