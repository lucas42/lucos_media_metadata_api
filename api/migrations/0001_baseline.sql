-- Baseline schema: current live shape after all one-off migrations are complete.
-- Uses CREATE TABLE IF NOT EXISTS so that:
--   - a blank environment creates every table on first boot, then records 0001;
--   - an existing production DB (already at this shape) no-ops the creates and
--     just records 0001, with data untouched. No manual stamping needed.

CREATE TABLE IF NOT EXISTS "track" (
	"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	"fingerprint" TEXT UNIQUE,
	"url" TEXT UNIQUE,
	"duration" INTEGER,
	"weighting" FLOAT NOT NULL DEFAULT 0,
	"cum_weighting" FLOAT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS "predicate" (
	"id" TEXT PRIMARY KEY NOT NULL
);

CREATE TABLE IF NOT EXISTS "tag" (
	"trackid" TEXT NOT NULL,
	"predicateid" TEXT NOT NULL,
	"value" TEXT,
	"uri" TEXT DEFAULT "",
	FOREIGN KEY (trackid) REFERENCES track(id),
	FOREIGN KEY (predicateid) REFERENCES predicate(id)
);

CREATE TABLE IF NOT EXISTS "collection" (
	"slug" TEXT PRIMARY KEY NOT NULL,
	"name" TEXT UNIQUE NOT NULL,
	"icon" TEXT DEFAULT ""
);

CREATE TABLE IF NOT EXISTS "collection_track" (
	"collectionslug" TEXT NOT NULL,
	"trackid" TEXT NOT NULL,
	FOREIGN KEY (collectionslug) REFERENCES collection(slug),
	FOREIGN KEY (trackid) REFERENCES track(id),
	CONSTRAINT track_collection_unique UNIQUE (collectionslug, trackid)
);

CREATE TABLE IF NOT EXISTS "album" (
	"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	"name" TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS "artist" (
	"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	"name" TEXT NOT NULL UNIQUE
);
