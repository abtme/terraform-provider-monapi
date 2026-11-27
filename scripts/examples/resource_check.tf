resource "monapi_check" "website" {
  name          = "Website"
  kind          = "http"
  target        = "https://www.example.com/"
  expect_status = 200
  expect_text   = "Welcome"
  channel_ids   = [monapi_channel.ops.id]
}

resource "monapi_check" "mail" {
  name             = "Mail server"
  kind             = "smtp"
  target           = "mail.example.com"
  require_starttls = true
}

resource "monapi_check" "certificate" {
  name             = "Certificate"
  kind             = "tls"
  target           = "www.example.com"
  expire_warn_days = 21
}

resource "monapi_check" "backup" {
  name             = "Nightly backup"
  kind             = "heartbeat"
  interval_seconds = 86400
  grace_seconds    = 3600
}

# The backup job calls this when it has finished.
output "backup_heartbeat_url" {
  value = monapi_check.backup.heartbeat_url
}

resource "monapi_check" "server" {
  name         = "web1"
  kind         = "agent"
  services     = ["nginx.service", "postgresql.service"]
  ports        = [443]
  disk_percent = 85
}

# For /etc/monapi/monapi-agent.env on web1 (MONAPI_AGENT_KEY).
output "web1_agent_key" {
  value     = monapi_check.server.agent_key
  sensitive = true
}
