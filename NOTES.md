# Notes

## 2026-11-27: first version (monapi M7)

- **Resources:** `monapi_check`, `monapi_channel`, `monapi_status_page`,
  `monapi_status_page_domain_verification`, `monapi_maintenance`; data
  source `monapi_workspace`.
- **One flat check resource** with every kind's settings as top-level
  optional attributes. ValidateConfig refuses settings the kind does not
  use, so a value is never silently dropped by the server (Terraform would
  then fail with "inconsistent result after apply").
- **Server defaults:** optional values are Optional+Computed and take
  what the server fills in (interval, thresholds, port 25/443, record type
  A, warn days, agent limits). The usual Optional+Computed rule follows:
  removing a value from the configuration keeps the last one; to go back
  to a default, set it.
- **Normalised input:** the server trims targets and lowers domain names;
  the provider keeps the spelling that was written when it means the same
  (a test applies `Example.COM` and then plans nothing).
- **Channel secrets are write-only** (sealed on the server, never
  returned), so the provider keeps them as configured; changes made
  elsewhere are not seen, and an import cannot fill them.
- **Maintenance cannot be changed on the server**, so every attribute
  forces replacement. Ended maintenance drops off the server's list after
  30 days; the provider then keeps it in the state instead of planning it
  again.
- **Domain verification** is its own resource so it can depend on the TXT
  record made by a DNS provider; it fails (with the server's reason) until
  the record is visible.
- **Tests:** acceptance tests against a real monapi server and PostgreSQL
  (scripts/acc-test.sh): create, no-drift plan, update in place or replace
  as expected, import, removal outside Terraform, validation errors, the
  public page shown and hidden; nothing left behind afterwards. Run with
  both Terraform 1.16 and OpenTofu. Found while writing them: an imported
  channel left webhook_secret null (fixed), and the default for
  locations_required is 2 (the test had it wrong).
- **monapi side:** `monapi-server token create` (added with this
  provider) makes the first token without a browser.
- **Release** needs, like the other providers: the `arc-runners-monapi`
  runner scale set (talos repo), an OpenBao JWT role
  `terraform-provider-monapi-release`, and the repo made public for the
  Terraform Registry. None of that is done yet.
