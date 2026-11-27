#!/bin/sh
# Acceptance tests against a real monapi server: builds monapi-server from
# MONAPI_SRC (default ../monapi), resets the throwaway database
# MONAPI_ACC_DATABASE_URL, makes an account, workspace and owner token,
# starts the server, and runs the TestAcc tests with Terraform.
#   MONAPI_ACC_DATABASE_URL=postgres://... sh scripts/acc-test.sh [go test args]
set -eu
cd "$(dirname "$0")/.."
: "${MONAPI_ACC_DATABASE_URL:?set MONAPI_ACC_DATABASE_URL to a throwaway database (it is emptied)}"
src="${MONAPI_SRC:-../monapi}"
port="${PORT:-18491}"
tmp="$(mktemp -d)"
pid=""
trap '[ -n "$pid" ] && kill "$pid" 2>/dev/null; rm -rf "$tmp"' EXIT

(cd "$src" && go build -o "$tmp/" ./cmd/monapi-server)
export MONAPI_DATABASE_URL="$MONAPI_ACC_DATABASE_URL"
export MONAPI_PUBLIC_URL="http://127.0.0.1:$port" MONAPI_LISTEN="127.0.0.1:$port"
MONAPI_SECRET_KEY="$("$tmp/monapi-server" gen-key)"
export MONAPI_SECRET_KEY
PGOPTIONS='-c client_min_messages=warning' psql "$MONAPI_DATABASE_URL" -qc 'DROP SCHEMA public CASCADE; CREATE SCHEMA public' >/dev/null
"$tmp/monapi-server" user create --email acc@example.com --name "Acceptance" --workspace "Acc test" >/dev/null 2>&1
MONAPI_TOKEN="$("$tmp/monapi-server" token create --email acc@example.com --workspace acc-test --name terraform --role owner 2>/dev/null | grep -o 'mnp_[A-Za-z0-9_-]*')"
[ -n "$MONAPI_TOKEN" ] || { echo "acc-test: no token made" >&2; exit 1; }
export MONAPI_TOKEN MONAPI_URL="$MONAPI_PUBLIC_URL"

"$tmp/monapi-server" > "$tmp/server.log" 2>&1 &
pid=$!
i=0
until curl -sf "$MONAPI_URL/healthz" >/dev/null; do
	i=$((i + 1)); [ "$i" -lt 50 ] || { cat "$tmp/server.log"; exit 1; }; sleep 0.2
done

status=0
TF_ACC=1 go test -count=1 -timeout 15m ./internal/provider/ "$@" || status=$?
if [ "$status" -ne 0 ]; then
	echo "--- server log (errors) ---"; grep -E '"level":"(ERROR|WARN)"' "$tmp/server.log" | tail -20 || true
fi
exit "$status"
