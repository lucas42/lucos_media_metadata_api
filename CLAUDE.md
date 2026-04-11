# lucos_media_metadata_api

## Data Integrity Anti-Pattern

**Do not encode non-URI values to make them look like valid IRIs.**

For predicates like `about` and `mentions`, the `uri` column in the `tag` table must contain a real URI (e.g. an eolas entity URI like `https://eolas.l42.eu/metadata/person/alice/`). If a record has a non-URI value in a URI field, encoding it with `url.PathEscape` or similar does not fix the data — it papers over the problem.

This encoding approach has been proposed and rejected multiple times. When invalid data appears in a URI field, the correct response is to:
1. Investigate how the invalid data got there (which write path, migration gap, or data entry issue)
2. Fix the root cause so it cannot happen again
3. Clean up the existing bad data with a targeted migration

## Run Tests

```bash
cd api && /usr/local/go/bin/go test ./...
```

## Test Artifacts

Running tests leaves behind `*.sqlite-shm`, `*.sqlite-wal`, a built `api/api` binary, and a `migrate_album_tags` binary in the repo root. These are covered by `.gitignore` — always run `git status` before `git add -A` to avoid accidentally staging them.
