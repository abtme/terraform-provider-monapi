# Terraform provider for monapi

Manages a monapi workspace as code: checks
(websites, ports, mail servers, DNS, certificates, domains, blacklists,
heartbeats, server agents), notification channels, public status pages
(with custom domain verification) and planned maintenance.

Works with Terraform and OpenTofu.

```terraform
terraform {
  required_providers {
    monapi = { source = "abtme/monapi" }
  }
}

provider "monapi" {
  url   = "https://monitor.example.com" # or MONAPI_URL
  token = var.monapi_token              # or MONAPI_TOKEN
}

resource "monapi_check" "website" {
  name   = "Website"
  kind   = "http"
  target = "https://www.example.com/"
}
```

Each API token belongs to one workspace. Make one in the workspace's
Settings, or on the server:

```sh
monapi-server token create --email you@example.com --workspace ops --name terraform --role owner
```

Checks and status pages need an editor token; channels need an owner
token. Full reference: [docs/](docs/index.md).

## Developing

```sh
sh scripts/check.sh                       # gofmt, vet, unit tests, examples, docs
MONAPI_ACC_DATABASE_URL=postgres://... \
  sh scripts/check.sh                     # plus acceptance tests (a throwaway database: it is emptied)
sh scripts/docs.sh                        # regenerate docs/ after a schema change
```

The acceptance tests build `monapi-server` from `../monapi` (or
`MONAPI_SRC`), start it on that database, and drive Terraform (or OpenTofu:
`TF_ACC_TERRAFORM_PATH=$(command -v tofu)`) against it.

Releases: push a tag `vX.Y.Z`; `.github/workflows/release.yml` builds,
signs and publishes the release for the Terraform Registry.

## Licence

Apache-2.0. Copyright 2026 Andrew Turnbull.
