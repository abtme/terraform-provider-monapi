#!/bin/sh
# Builds the provider, validates the examples against its schema (with a
# dev_overrides CLI config, nothing downloaded), and writes docs/ from the
# schema. CHECK=1: fail if docs/ would change (for CI).
set -eu
cd "$(dirname "$0")/.."
tf="${TF:-$(command -v tofu || command -v terraform)}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/bin/terraform-provider-monapi" .
cat > "$tmp/tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/abtme/monapi" = "$tmp/bin"
    "registry.opentofu.org/abtme/monapi" = "$tmp/bin"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$tmp/tfrc" TF_IN_AUTOMATION=1
mkdir "$tmp/ex"
# The domain verification example uses a DNS provider; it is not validated here.
for f in scripts/examples/*.tf; do
	case "$f" in *domain_verification*) continue ;; esac
	cp "$f" "$tmp/ex/"
done
(cd "$tmp/ex" && "$tf" validate -no-color >/dev/null 2>"$tmp/validate.err") || { cat "$tmp/validate.err"; exit 1; }
(cd "$tmp/ex" && "$tf" providers schema -json) > "$tmp/schema.json" 2>/dev/null
if [ "${CHECK:-}" = 1 ]; then
	cp -r docs "$tmp/docs-before"
	python3 -I scripts/gendocs.py "$tmp/schema.json"
	diff -r "$tmp/docs-before" docs || { echo "docs/ is out of date: run sh scripts/docs.sh" >&2; exit 1; }
else
	python3 -I scripts/gendocs.py "$tmp/schema.json"
fi
echo "examples valid, docs written"
