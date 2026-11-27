#!/bin/sh
# Everything before a commit: formatting, vet, unit tests, examples and
# docs up to date, and (with MONAPI_ACC_DATABASE_URL) the acceptance tests
# against a real monapi server.
set -eu
cd "$(dirname "$0")/.."
test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt needed" >&2; exit 1; }
go mod tidy -diff
go vet ./...
go test -count=1 ./...
CHECK=1 sh scripts/docs.sh
if [ -n "${MONAPI_ACC_DATABASE_URL:-}" ]; then
	sh scripts/acc-test.sh
else
	echo "acceptance tests skipped (set MONAPI_ACC_DATABASE_URL)"
fi
echo "check OK"
