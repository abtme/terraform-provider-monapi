resource "monapi_status_page" "public" {
  slug          = "example"
  title         = "Example status"
  custom_domain = "status.example.com"
  components = [
    { check_id = monapi_check.website.id, name = "Website" },
    { check_id = monapi_check.mail.id, name = "Email" },
  ]
}
