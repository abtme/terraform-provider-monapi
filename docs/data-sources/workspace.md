---
page_title: "monapi_workspace Data Source - monapi"
subcategory: ""
description: |-
  The workspace the provider's token belongs to, and what the token may do there.
---

# monapi_workspace (Data Source)

The workspace the provider's token belongs to, and what the token may do there.

## Example Usage

```terraform
data "monapi_workspace" "this" {}

output "workspace" {
  value = "${data.monapi_workspace.this.name} (token role: ${data.monapi_workspace.this.role})"
}
```

## Schema

### Read-Only

- `id` (String)
- `name` (String)
- `role` (String) viewer, editor or owner.
- `slug` (String)
