---
page_title: "monapi_maintenance Resource - monapi"
subcategory: ""
description: |-
  Planned maintenance: while it runs its checks notify nobody (what happens is still recorded), and a status page can show it in advance. Maintenance cannot be changed, only replaced, so any change cancels it and plans it again. Maintenance that ended more than 30 days ago is no longer listed by the server; it stays in the state until removed from the configuration.
---

# monapi_maintenance (Resource)

Planned maintenance: while it runs its checks notify nobody (what happens is still recorded), and a status page can show it in advance. Maintenance cannot be changed, only replaced, so any change cancels it and plans it again. Maintenance that ended more than 30 days ago is no longer listed by the server; it stays in the state until removed from the configuration.

## Example Usage

```terraform
resource "monapi_maintenance" "db_upgrade" {
  title     = "Database upgrade"
  message   = "Email may be delayed for up to an hour."
  page_id   = monapi_status_page.public.id
  starts_at = "2026-12-06T02:00:00Z"
  ends_at   = "2026-12-06T04:00:00Z"
  check_ids = [monapi_check.mail.id]
}
```

## Schema

### Required

- `check_ids` (Set of String)
- `ends_at` (String) After starts_at, at most 30 days later.
- `starts_at` (String)
- `title` (String)

### Optional

- `message` (String) For the status page.
- `page_id` (String) The status page to show it on (optional).

### Read-Only

- `id` (String)

## Import

Import is supported using the following syntax:

```shell
terraform import monapi_maintenance.example <maintenance_id>
```
