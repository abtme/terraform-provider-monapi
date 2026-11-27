---
page_title: "monapi_check Resource - monapi"
subcategory: ""
description: |-
  A check: something monapi watches (a website, port, mail server, DNS answer, certificate, domain, blacklist listing, heartbeat or server agent). Unset optional values take the server's defaults; to go back to a default after setting a value, set the default explicitly.
---

# monapi_check (Resource)

A check: something monapi watches (a website, port, mail server, DNS answer, certificate, domain, blacklist listing, heartbeat or server agent). Unset optional values take the server's defaults; to go back to a default after setting a value, set the default explicitly.

## Example Usage

```terraform
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
```

## Schema

### Required

- `kind` (String) One of http, tcp, ping, smtp, dns, heartbeat, tls, blacklist, domain, agent. Changing it makes a new check.
- `name` (String) Shown in notifications and the app (1 to 100 characters).

### Optional

- `channel_ids` (Set of String) Channels to notify besides those set to all checks.
- `disk_percent` (Number) agent: a filesystem fuller than this is a problem (default 90).
- `expect_status` (Number) http: the status that counts as up (default any 2xx or 3xx).
- `expect_text` (String) http: text that must be in the page.
- `expect_value` (String) dns: one answer must equal this.
- `expire_warn_days` (Number) tls (default 14) and domain (default 30): fail this many days before expiry.
- `failures_before_down` (Number) Failures in a row (per location) before a location counts it down; 1 to 10.
- `follow_redirects` (Boolean) http: follow redirects (default false: a redirect is the answer).
- `grace_seconds` (Number) Heartbeat: how late a check-in may be.
- `interval_seconds` (Number) How often to check (minimum and default depend on the kind; up to 86400). For a heartbeat, how often the job should check in.
- `lists` (List of String) blacklist: DNSBL zones to ask (default a well-known set), lower case.
- `locations_required` (Number) How many probe locations must agree it is down; 1 to 20.
- `mem_percent` (Number) agent: memory use above this is a problem (default 95).
- `paused` (Boolean) A paused check is not run and notifies nobody.
- `port` (Number) tcp (required), smtp (default 25), tls (default 443).
- `ports` (List of Number) agent: TCP ports something must listen on.
- `processes` (List of String) agent: processes that must be running (as ps shows them, or the program or script file name).
- `record_type` (String) dns: A (default), AAAA, CNAME, MX, TXT or NS.
- `require_starttls` (Boolean) smtp: the server must offer STARTTLS.
- `resolver` (String) dns: the resolver to ask, host:port (default the probe's own).
- `server_name` (String) tls: the name the certificate must be for (default the target).
- `services` (List of String) agent: systemd units that must be active.
- `skip_tls_verify` (Boolean) http: accept any certificate.
- `smart` (Boolean) agent: ask SMART health (the agent needs smartctl through sudo).
- `successes_before_up` (Number) Successes in a row before it counts as up again; 1 to 10.
- `target` (String) A URL (http), host name or address (tcp, ping, smtp, tls, blacklist), name (dns) or domain (domain). Not used by heartbeat and agent checks.
- `timeout_seconds` (Number) 1 to 60, shorter than the interval.

### Read-Only

- `agent_key` (String) agent: the key the agent reports with. (sensitive)
- `heartbeat_url` (String) heartbeat: the URL the job calls.
- `id` (String)
- `state` (String) up, down, unknown or paused, when last read.

## Import

Import is supported using the following syntax:

```shell
terraform import monapi_check.example <check_id>
```
