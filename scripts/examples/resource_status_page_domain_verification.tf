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
