#!/usr/bin/env python3
"""Generate Terraform Registry docs from the provider schema.

  sh scripts/docs.sh   # builds the provider, reads its schema, writes docs/

tfplugindocs needs a terraform binary resolving the provider from a registry, which
does not work for a provider that is not published yet; this reads the same schema
directly. Hand-written examples live in scripts/examples/<kind>_<name>.tf (scripts/docs.sh validates them).
"""
import json, os, sys

PROVIDER = "monapi"
ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")

IMPORT = {
    "check": "terraform import monapi_check.example <check_id>",
    "channel": "terraform import monapi_channel.example <channel_id>",
    "status_page": "terraform import monapi_status_page.example <status_page_id>",
    "maintenance": "terraform import monapi_maintenance.example <maintenance_id>",
}


def type_name(t):
    if isinstance(t, str):
        return {"string": "String", "number": "Number", "bool": "Boolean"}.get(t, t)
    kind, inner = t[0], t[1]
    word = {"list": "List of", "set": "Set of", "map": "Map of"}.get(kind, kind)
    return f"{word} {type_name(inner)}"


def attr_lines(name, a, level=0):
    flags = []
    if a.get("sensitive"):
        flags.append("sensitive")
    desc = (a.get("description") or "").strip()
    t = a.get("type")
    if "nested_type" in a:
        t = "List of Object"
    line = f"- `{name}` ({type_name(t) if isinstance(t, (str, list)) else t})"
    if desc:
        line += f" {desc}"
    if flags:
        line += f" ({', '.join(flags)})"
    return line


def schema_section(block, path_label=None):
    attrs = block.get("attributes", {})
    req = {k: v for k, v in attrs.items() if v.get("required")}
    opt = {k: v for k, v in attrs.items() if v.get("optional")}
    ro = {k: v for k, v in attrs.items() if v.get("computed") and not v.get("optional") and not v.get("required")}
    out = ["## Schema", ""]
    nested = []
    for title, group in (("Required", req), ("Optional", opt), ("Read-Only", ro)):
        if not group:
            continue
        out += [f"### {title}", ""]
        for k in sorted(group):
            out.append(attr_lines(k, group[k]))
            if "nested_type" in group[k]:
                nested.append((k, group[k]["nested_type"]))
        out.append("")
    for k, nt in nested:
        out += [f"<a id=\"nestedatt--{k}\"></a>", f"### Nested Schema for `{k}`", ""]
        for nk in sorted(nt["attributes"]):
            na = nt["attributes"][nk]
            word = "required" if na.get("required") else "optional" if na.get("optional") else "read-only"
            out.append(attr_lines(nk, na) + f" ({word})")
        out.append("")
    return "\n".join(out)


def example(kind, name):
    p = os.path.join(ROOT, "scripts", "examples", f"{kind}_{name}.tf")
    return open(p).read().strip() if os.path.exists(p) else None


def page(kind, name, block):
    full = f"{PROVIDER}_{name}"
    label = "Resource" if kind == "resource" else "Data Source"
    desc = (block.get("description") or "").strip()
    first = desc.split("\n")[0]
    out = ["---", f'page_title: "{full} {label} - {PROVIDER}"', 'subcategory: ""', "description: |-", f"  {first}", "---", "",
           f"# {full} ({label})", "", desc, ""]
    ex = example(kind, name)
    if ex:
        out += ["## Example Usage", "", "```terraform", ex, "```", ""]
    out.append(schema_section(block))
    if kind == "resource" and name in IMPORT:
        out += ["## Import", "", "Import is supported using the following syntax:", "", "```shell", IMPORT[name], "```", ""]
    return "\n".join(out).rstrip() + "\n"


def main(path):
    schemas = json.load(open(path))["provider_schemas"]
    d = next(v for k, v in schemas.items() if k.endswith("/abtme/monapi"))  # terraform or tofu registry
    os.makedirs(os.path.join(ROOT, "docs", "resources"), exist_ok=True)
    os.makedirs(os.path.join(ROOT, "docs", "data-sources"), exist_ok=True)
    for name, s in d["resource_schemas"].items():
        short = name[len(PROVIDER) + 1:]
        open(os.path.join(ROOT, "docs", "resources", f"{short}.md"), "w").write(page("resource", short, s["block"]))
    for name, s in d["data_source_schemas"].items():
        short = name[len(PROVIDER) + 1:]
        open(os.path.join(ROOT, "docs", "data-sources", f"{short}.md"), "w").write(page("data_source", short, s["block"]))
    idx = open(os.path.join(ROOT, "scripts", "examples", "provider.tf")).read().strip()
    prov = d["provider"]["block"]
    out = ["---", 'page_title: "Provider: monapi"', 'description: |-',
           "  Manage a monapi workspace: checks, notification channels, status pages and maintenance.", "---", "",
           "# monapi Provider", "", prov["description"], "",
           "## Example Usage", "", "```terraform", idx, "```", "",
           "## Notes", "",
           "- Make the token in the workspace's Settings, or on the server with `monapi-server token create`.",
           "- Checks and status pages need an editor token; channels need an owner token.",
           "- Channel settings (addresses, bot tokens, webhook URLs) are sealed by the server and never returned, so changes made outside Terraform are not seen.",
           "- Optional values left unset take the server's defaults; to go back to a default after setting a value, set the default.",
           "", schema_section(prov)]
    open(os.path.join(ROOT, "docs", "index.md"), "w").write("\n".join(out).rstrip() + "\n")


main(sys.argv[1])
