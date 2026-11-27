---
page_title: "monapi_status_page_domain_verification Resource - monapi"
subcategory: ""
description: |-
  Verifies a status page's custom domain: monapi looks for the page's domain_token in a TXT record at _monapi.<custom_domain>. Make the record first (depends_on it, or reference it). Creating this resource fails while the record is not visible yet; apply again once DNS has it. A changed custom_domain makes a new verification. Removing this resource does not undo the verification.
---

# monapi_status_page_domain_verification (Resource)

Verifies a status page's custom domain: monapi looks for the page's domain_token in a TXT record at _monapi.<custom_domain>. Make the record first (depends_on it, or reference it). Creating this resource fails while the record is not visible yet; apply again once DNS has it. A changed custom_domain makes a new verification. Removing this resource does not undo the verification.

## Example Usage

```terraform
# The TXT record, here with the Cloudflare provider (any DNS provider works).
resource "cloudflare_dns_record" "monapi_verify" {
  zone_id = var.zone_id
  name    = "_monapi.status.example.com"
  type    = "TXT"
  content = monapi_status_page.public.domain_token
  ttl     = 60
}

resource "monapi_status_page_domain_verification" "public" {
  page_id       = monapi_status_page.public.id
  custom_domain = monapi_status_page.public.custom_domain
  depends_on    = [cloudflare_dns_record.monapi_verify]
}
```

## Schema

### Required

- `custom_domain` (String) The page's custom_domain (pass it through so a change verifies again).
- `page_id` (String)

### Read-Only

- `id` (String)
- `verified` (Boolean)
