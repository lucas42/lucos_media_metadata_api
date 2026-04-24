#!/bin/sh
set -e

# Run an initial export on container start so the healthcheck has fresh output
# to verify within seconds, rather than waiting up to an hour for the first
# cron tick. Non-fatal: if this run fails, cron will retry on schedule and the
# healthcheck will stay unhealthy until a successful run produces a fresh file.
/usr/local/bin/rdf-exporter || echo "Initial RDF export failed; cron will retry on schedule"

exec crond -f -L /dev/stdout
