package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/abtme/terraform-provider-monapi/internal/client"
)

// These run against a real monapi server: scripts/acc-test.sh starts one on
// a throwaway database and sets MONAPI_URL and MONAPI_TOKEN (an owner token).

var factories = map[string]func() (tfprotov6.ProviderServer, error){
	"monapi": providerserver.NewProtocol6WithError(New("test")()),
}

func preCheck(t *testing.T) {
	if os.Getenv("MONAPI_URL") == "" || os.Getenv("MONAPI_TOKEN") == "" {
		t.Fatal("MONAPI_URL and MONAPI_TOKEN must be set (scripts/acc-test.sh does it)")
	}
}

func apiClient(t *testing.T) *client.Client {
	c, err := client.New(os.Getenv("MONAPI_URL"), os.Getenv("MONAPI_TOKEN"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAccWorkspace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheck(t) }, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: `data "monapi_workspace" "this" {}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.monapi_workspace.this", "slug", "acc-test"),
				resource.TestCheckResourceAttr("data.monapi_workspace.this", "role", "owner"),
			),
		}},
	})
}

func TestAccChecks(t *testing.T) {
	cfg := func(name string, status int) string {
		return fmt.Sprintf(`
resource "monapi_check" "web" {
  name          = %q
  kind          = "http"
  target        = "https://www.example.com/"
  expect_status = %d
  expect_text   = "Example"
}
resource "monapi_check" "backup" {
  name             = "Nightly backup"
  kind             = "heartbeat"
  interval_seconds = 86400
  grace_seconds    = 3600
}
resource "monapi_check" "server" {
  name         = "srv1"
  kind         = "agent"
  services     = ["nginx.service", "postgresql.service"]
  processes    = ["php-fpm"]
  ports        = [443, 25]
  disk_percent = 85
}
resource "monapi_check" "mx" {
  name        = "MX record"
  kind        = "dns"
  target      = "Example.COM"
  record_type = "MX"
}
resource "monapi_check" "mail" {
  name   = "Mail server"
  kind   = "smtp"
  target = "mail.example.com"
}
`, name, status)
	}
	var webID string
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheck(t) }, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Website", 200),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("monapi_check.web", "id", uuidRe),
					resource.TestCheckResourceAttr("monapi_check.web", "interval_seconds", "60"),
					resource.TestCheckResourceAttr("monapi_check.web", "locations_required", "2"),
					resource.TestCheckResourceAttr("monapi_check.web", "paused", "false"),
					resource.TestCheckResourceAttr("monapi_check.web", "state", "unknown"),
					resource.TestMatchResourceAttr("monapi_check.backup", "heartbeat_url", regexp.MustCompile(`^http://127\.0\.0\.1:\d+/hb/mnh_`)),
					resource.TestCheckResourceAttr("monapi_check.backup", "interval_seconds", "86400"),
					resource.TestMatchResourceAttr("monapi_check.server", "agent_key", regexp.MustCompile(`^mna_`)),
					resource.TestCheckResourceAttr("monapi_check.server", "mem_percent", "95"),
					resource.TestCheckResourceAttr("monapi_check.server", "ports.1", "25"),
					resource.TestCheckResourceAttr("monapi_check.mx", "target", "Example.COM"),
					resource.TestCheckResourceAttr("monapi_check.mail", "port", "25"),
				),
			},
			// The same configuration plans nothing (no drift from server defaults or normalising).
			{Config: cfg("Website", 200), PlanOnly: true},
			{
				Config: cfg("Website (renamed)", 301),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("monapi_check.web", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("monapi_check.web", "name", "Website (renamed)"),
					resource.TestCheckResourceAttr("monapi_check.web", "expect_status", "301"),
					recordID("monapi_check.web", &webID),
				),
			},
			{ResourceName: "monapi_check.web", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"state"}},
			{ResourceName: "monapi_check.server", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"state"}},
			// Removed outside Terraform: planned again.
			{
				PreConfig: func() {
					if err := apiClient(t).DeleteCheck(context.Background(), webID); err != nil {
						t.Fatal(err)
					}
				},
				Config:             cfg("Website (renamed)", 301),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func recordID(name string, into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not in state", name)
		}
		*into = rs.Primary.ID
		return nil
	}
}

func TestAccCheckValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheck(t) }, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: `resource "monapi_check" "x" {
  name = "x"
  kind = "http"
  target = "https://example.com/"
  port = 8080
}`, ExpectError: regexp.MustCompile(`port is not used by http checks`)},
			{Config: `resource "monapi_check" "x" {
  name = "x"
  kind = "tcp"
  target = "example.com"
}`, ExpectError: regexp.MustCompile(`tcp checks need a port`)},
			{Config: `resource "monapi_check" "x" {
  name = "x"
  kind = "heartbeat"
  target = "example.com"
}`, ExpectError: regexp.MustCompile(`heartbeat checks have no target`)},
			// The server's own refusal reaches the user.
			{Config: `resource "monapi_check" "x" {
  name = "x"
  kind = "http"
  target = "ftp://example.com/"
}`, ExpectError: regexp.MustCompile(`target must be an http:// or https:// URL`)},
			{Config: `resource "monapi_channel" "x" {
  name = "x"
  kind = "telegram"
  chat_id = "-100123"
}`, ExpectError: regexp.MustCompile(`telegram channels need bot_token`)},
		},
	})
}

func TestAccChannels(t *testing.T) {
	cfg := func(name string, all bool) string {
		return fmt.Sprintf(`
resource "monapi_channel" "hook" {
  name       = %q
  kind       = "webhook"
  url        = "https://hooks.example.com/monapi"
  all_checks = %t
}
resource "monapi_channel" "mail" {
  name = "Ops email"
  kind = "email"
  to   = ["ops@example.com", "oncall@example.com"]
}
resource "monapi_check" "web" {
  name        = "Website"
  kind        = "http"
  target      = "https://www.example.com/"
  channel_ids = [monapi_channel.hook.id]
}
`, name, all)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheck(t) }, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Hook", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("monapi_channel.hook", "webhook_secret", regexp.MustCompile(`.{20,}`)),
					resource.TestCheckResourceAttr("monapi_channel.hook", "all_checks", "false"),
					resource.TestCheckResourceAttr("monapi_channel.mail", "all_checks", "true"),
					resource.TestMatchResourceAttr("monapi_channel.mail", "summary", regexp.MustCompile(`ops@example\.com`)),
					resource.TestCheckResourceAttr("monapi_check.web", "channel_ids.#", "1"),
					resource.TestCheckResourceAttrPair("monapi_check.web", "channel_ids.0", "monapi_channel.hook", "id"),
				),
			},
			{Config: cfg("Hook", false), PlanOnly: true},
			{
				Config: cfg("Hook renamed", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("monapi_channel.hook", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("monapi_channel.hook", "name", "Hook renamed"),
					resource.TestCheckResourceAttr("monapi_channel.hook", "all_checks", "true"),
				),
			},
			{ResourceName: "monapi_channel.mail", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"to"}},
		},
	})
}

func TestAccStatusPage(t *testing.T) {
	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Hour)
	cfg := func(order string, published bool, title string) string {
		return fmt.Sprintf(`
resource "monapi_check" "a" {
  name   = "internal-web-01"
  kind   = "ping"
  target = "192.0.2.10"
}
resource "monapi_check" "b" {
  name   = "internal-db-01"
  kind   = "ping"
  target = "192.0.2.11"
}
locals {
  comps = {
    a = { check_id = monapi_check.a.id, name = "Website" }
    b = { check_id = monapi_check.b.id, name = "Database" }
  }
}
resource "monapi_status_page" "acme" {
  slug          = "acme-acc"
  title         = "Acme status"
  custom_domain = "status.acme.test"
  published     = %t
  components    = [for k in %s : local.comps[k]]
}
resource "monapi_maintenance" "upgrade" {
  title     = %q
  page_id   = monapi_status_page.acme.id
  starts_at = %q
  ends_at   = %q
  check_ids = [monapi_check.b.id]
}
`, published, order, title, start.Format(time.RFC3339), start.Add(2*time.Hour).Format("2006-01-02T15:04:05-07:00"))
	}
	var maintID string
	resource.Test(t, resource.TestCase{
		PreCheck: func() { preCheck(t) }, ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: cfg(`["a", "b"]`, true, "Database upgrade"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("monapi_status_page.acme", "components.#", "2"),
					resource.TestCheckResourceAttr("monapi_status_page.acme", "components.0.name", "Website"),
					resource.TestCheckResourceAttr("monapi_status_page.acme", "domain_verified", "false"),
					resource.TestMatchResourceAttr("monapi_status_page.acme", "domain_token", regexp.MustCompile(`^monapi-verify-`)),
					resource.TestMatchResourceAttr("monapi_status_page.acme", "url", regexp.MustCompile(`/s/acme-acc$`)),
					resource.TestCheckResourceAttr("monapi_maintenance.upgrade", "check_ids.#", "1"),
					recordID("monapi_maintenance.upgrade", &maintID),
					publicPage("acme-acc", 200, "Planned maintenance: Database upgrade"),
				),
			},
			{Config: cfg(`["a", "b"]`, true, "Database upgrade"), PlanOnly: true},
			{
				Config: cfg(`["b", "a"]`, false, "Database upgrade (moved)"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("monapi_status_page.acme", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("monapi_maintenance.upgrade", plancheck.ResourceActionReplace),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("monapi_status_page.acme", "components.0.name", "Database"),
					resource.TestCheckResourceAttr("monapi_status_page.acme", "published", "false"),
					publicPage("acme-acc", 404, ""),
					func(s *terraform.State) error {
						if s.RootModule().Resources["monapi_maintenance.upgrade"].Primary.ID == maintID {
							return fmt.Errorf("maintenance was not replaced")
						}
						_, err := apiClient(t).GetMaintenance(context.Background(), maintID)
						if !client.IsNotFound(err) {
							return fmt.Errorf("the old maintenance is still there: %v", err)
						}
						return nil
					},
				),
			},
			{ResourceName: "monapi_status_page.acme", ImportState: true, ImportStateVerify: true},
			{ResourceName: "monapi_maintenance.upgrade", ImportState: true, ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{"ends_at"}}, // imported in UTC, configured with an offset
			// No TXT record exists, so verification fails with the server's reason.
			{
				Config: cfg(`["b", "a"]`, false, "Database upgrade (moved)") + `
resource "monapi_status_page_domain_verification" "acme" {
  page_id       = monapi_status_page.acme.id
  custom_domain = monapi_status_page.acme.custom_domain
}`,
				ExpectError: regexp.MustCompile(`_monapi\.status\.acme\.test`),
			},
		},
	})
}

func publicPage(slug string, want int, text string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		status, body, err := httpGet(os.Getenv("MONAPI_URL") + "/s/" + slug)
		if err != nil {
			return err
		}
		if status != want {
			return fmt.Errorf("/s/%s answered %d, want %d", slug, status, want)
		}
		if text != "" && !regexp.MustCompile(regexp.QuoteMeta(text)).MatchString(body) {
			return fmt.Errorf("/s/%s does not show %q", slug, text)
		}
		for _, secret := range []string{"192.0.2.10", "internal-web-01"} {
			if regexp.MustCompile(regexp.QuoteMeta(secret)).MatchString(body) {
				return fmt.Errorf("/s/%s shows %q", slug, secret)
			}
		}
		return nil
	}
}

func httpGet(url string) (int, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), err
}
