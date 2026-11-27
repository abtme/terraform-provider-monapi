---
page_title: "monapi_status_page Resource - monapi"
subcategory: ""
description: |-
  A public status page: chosen checks under public names, their uptime, incidents and planned maintenance. Incidents are posted by people (in the app or API), not managed here.
---

# monapi_status_page (Resource)

A public status page: chosen checks under public names, their uptime, incidents and planned maintenance. Incidents are posted by people (in the app or API), not managed here.

## Example Usage

```terraform
resource "monapi_status_page" "public" {
  slug          = "example"
  title         = "Example status"
  custom_domain = "status.example.com"
  components = [
    { check_id = monapi_check.website.id, name = "Website" },
    { check_id = monapi_check.mail.id, name = "Email" },
  ]
}
```

## Schema

### Required

- `slug` (String) The address: /s/<slug>. 3 to 50 of a-z, 0-9 and -.
- `title` (String)

### Optional

- `components` (List of Object) The checks shown, in this order, each under a public name (the check's own name and target are never shown).
- `custom_domain` (String) Serve the page on this name too, once verified (see monapi_status_page_domain_verification). Point it at the monapi server (CNAME) and give that server a certificate for it.
- `description` (String)
- `published` (Boolean) An unpublished page is not shown to anyone.

### Read-Only

- `domain_token` (String) Put this in a TXT record at _monapi.<custom_domain> to verify the domain.
- `domain_verified` (Boolean) Whether custom_domain has been verified (a changed domain must be verified again).
- `id` (String)
- `url` (String) The page's address on the monapi server.

<a id="nestedatt--components"></a>
### Nested Schema for `components`

- `check_id` (String) (required)
- `name` (String) (required)

## Import

Import is supported using the following syntax:

```shell
terraform import monapi_status_page.example <status_page_id>
```
