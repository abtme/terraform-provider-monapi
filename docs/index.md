---
page_title: "Provider: monapi"
description: |-
  Manage a monapi workspace: checks, notification channels, status pages and maintenance.
---

# monapi Provider

Manage a monapi workspace: checks, notification channels, status pages and maintenance. Each API token belongs to one workspace; use one provider block (or alias) per workspace.

## Example Usage

```terraform
terraform {
  required_providers {
    monapi = {
      source = "abtme/monapi"
    }
  }
}

# Or set MONAPI_URL and MONAPI_TOKEN.
provider "monapi" {
  url   = "https://monitor.example.com"
  token = var.monapi_token
}

variable "monapi_token" {
  type      = string
  sensitive = true
}
```

## Notes

- Make the token in the workspace's Settings, or on the server with `monapi-server token create`.
- Checks and status pages need an editor token; channels need an owner token.
- Channel settings (addresses, bot tokens, webhook URLs) are sealed by the server and never returned, so changes made outside Terraform are not seen.
- Optional values left unset take the server's defaults; to go back to a default after setting a value, set the default.

## Schema

### Optional

- `token` (String) A workspace API token (mnp_...), made under the workspace's Settings. Or MONAPI_TOKEN. Use an editor token (owner to manage channels). (sensitive)
- `url` (String) The monapi server, such as https://monitor.example.com. Or MONAPI_URL.
