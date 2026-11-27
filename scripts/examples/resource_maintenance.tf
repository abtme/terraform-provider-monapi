resource "monapi_maintenance" "db_upgrade" {
  title     = "Database upgrade"
  message   = "Email may be delayed for up to an hour."
  page_id   = monapi_status_page.public.id
  starts_at = "2026-12-06T02:00:00Z"
  ends_at   = "2026-12-06T04:00:00Z"
  check_ids = [monapi_check.mail.id]
}
