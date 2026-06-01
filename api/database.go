package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"sync/atomic"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

/**
 * A struct for wrapping a database
 */
type Datastore struct {
	DB *sqlx.DB
	Loganne LoganneInterface
	ManagerOrigin string
	// infoCache holds a pointer to the most recently computed /_info metrics snapshot.
	// It is a pointer to an atomic so it remains valid when Datastore is copied by value.
	infoCache *atomic.Pointer[InfoMetricsSnapshot]
}

func DBInit(dbpath string, loganne LoganneInterface) (database Datastore) {
	db := sqlx.MustConnect("sqlite3", dbpath+"?_busy_timeout=10000")
	database = Datastore{DB: db, Loganne: loganne, infoCache: new(atomic.Pointer[InfoMetricsSnapshot])}
	database.DB.MustExec("PRAGMA journal_mode=WAL;")
	database.DB.MustExec("PRAGMA foreign_keys = ON;")
	database.applyMigrations()
	return
}

// applyMigrations applies all pending embedded SQL migration files in lexical order.
func (store Datastore) applyMigrations() {
	store.applyMigrationsFromFS(migrationsFS, "migrations/*.sql")
}

// applyMigrationsFromFS is the testable core of the migration runner.
// It creates the schema_migrations table if absent, then applies any not-yet-recorded
// migrations matched by pattern in migFS, in lexical order.
// Each migration and its schema_migrations record are written in a single transaction
// so "applied" and "recorded" are always atomic. Any failure panics — the process must
// not start serving traffic on a half-migrated schema.
func (store Datastore) applyMigrationsFromFS(migFS fs.FS, pattern string) {
	store.DB.MustExec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	files, err := fs.Glob(migFS, pattern)
	if err != nil {
		panic(err)
	}
	sort.Strings(files)
	for _, f := range files {
		version := filepath.Base(f)
		// SELECT EXISTS always returns exactly one row (0 or 1); no sql.ErrNoRows
		// special-casing, and no driver-dependent integer-to-bool coercion.
		var exists int
		if err := store.DB.Get(&exists, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", version); err != nil {
			panic(err)
		}
		if exists == 1 {
			continue
		}
		sqlText, err := fs.ReadFile(migFS, f)
		if err != nil {
			panic(err)
		}
		slog.Info("Applying migration", slog.String("version", version))
		tx := store.DB.MustBegin()
		tx.MustExec(string(sqlText))
		tx.MustExec("INSERT INTO schema_migrations (version) VALUES (?)", version)
		if err := tx.Commit(); err != nil {
			panic(err) // fail loud; do not start half-migrated
		}
	}
}

