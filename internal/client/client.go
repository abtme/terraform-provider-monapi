// Package client talks to the monapi API (the workspace endpoints a
// workspace API token may use).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is one monapi server and API token (the token's workspace).
type Client struct {
	base   string
	token  string
	http   *http.Client
	agent  string
	debugf func(string, ...any)
}

// Option changes a Client.
type Option func(*Client)

// WithHTTPClient replaces the HTTP client (tests).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithDebug logs each request.
func WithDebug(f func(string, ...any)) Option { return func(c *Client) { c.debugf = f } }

// WithUserAgent sets the User-Agent.
func WithUserAgent(ua string) Option { return func(c *Client) { c.agent = ua } }

// New returns a client for the server at baseURL.
func New(baseURL, token string, opts ...Option) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("monapi URL must be http(s)://host, got %q", baseURL)
	}
	if !strings.HasPrefix(token, "mnp_") {
		return nil, errors.New("the API token must be a monapi workspace token (mnp_...)")
	}
	c := &Client{base: u.String(), token: token, http: &http.Client{Timeout: 60 * time.Second}, agent: "terraform-provider-monapi",
		debugf: func(string, ...any) {}}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// Error is the server's error answer.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("monapi answered %d", e.Status)
	}
	return fmt.Sprintf("monapi answered %d %s: %s", e.Status, e.Code, e.Message)
}

// IsNotFound reports whether err is a 404 (the object is gone).
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.agent)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.debugf("%s %s", method, path)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		e := &Error{Status: resp.StatusCode}
		var b struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &b) == nil {
			e.Code, e.Message = b.Error.Code, b.Error.Message
		}
		return e
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("monapi's answer to %s %s: %w", method, path, err)
	}
	return nil
}

func esc(id string) string { return url.PathEscape(id) }

// --- workspace ------------------------------------------------------------------------

// Workspace is the token's workspace.
type Workspace struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Workspace returns the token's workspace.
func (c *Client) Workspace(ctx context.Context) (Workspace, error) {
	var w Workspace
	return w, c.do(ctx, "GET", "/v1/workspace", nil, &w)
}

// --- checks ------------------------------------------------------------------------------

// CheckSettings are a check's kind-specific settings.
type CheckSettings struct {
	ExpectStatus    int      `json:"expect_status,omitempty"`
	ExpectText      string   `json:"expect_text,omitempty"`
	FollowRedirects bool     `json:"follow_redirects,omitempty"`
	SkipTLSVerify   bool     `json:"skip_tls_verify,omitempty"`
	RequireSTARTTLS bool     `json:"require_starttls,omitempty"`
	RecordType      string   `json:"record_type,omitempty"`
	ExpectValue     string   `json:"expect_value,omitempty"`
	Resolver        string   `json:"resolver,omitempty"`
	ExpireWarnDays  int      `json:"expire_warn_days,omitempty"`
	ServerName      string   `json:"server_name,omitempty"`
	Lists           []string `json:"lists,omitempty"`
	Services        []string `json:"services,omitempty"`
	Processes       []string `json:"processes,omitempty"`
	Ports           []int    `json:"ports,omitempty"`
	DiskPercent     int      `json:"disk_percent,omitempty"`
	MemPercent      int      `json:"mem_percent,omitempty"`
	SMART           bool     `json:"smart,omitempty"`
}

// CheckWrite is what a check is created or replaced with.
type CheckWrite struct {
	Name               string        `json:"name"`
	Kind               string        `json:"kind"`
	Target             string        `json:"target"`
	Port               int           `json:"port,omitempty"`
	Settings           CheckSettings `json:"settings"`
	IntervalSeconds    int           `json:"interval_seconds,omitempty"`
	TimeoutSeconds     int           `json:"timeout_seconds,omitempty"`
	GraceSeconds       int           `json:"grace_seconds,omitempty"`
	FailuresBeforeDown int           `json:"failures_before_down,omitempty"`
	SuccessesBeforeUp  int           `json:"successes_before_up,omitempty"`
	LocationsRequired  int           `json:"locations_required,omitempty"`
	Paused             bool          `json:"paused,omitempty"`
	ChannelIDs         *[]string     `json:"channel_ids,omitempty"`
}

// Check is a check as the server has it (defaults filled in).
type Check struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	Kind               string        `json:"kind"`
	Target             string        `json:"target"`
	Port               int           `json:"port"`
	Settings           CheckSettings `json:"settings"`
	IntervalSeconds    int           `json:"interval_seconds"`
	TimeoutSeconds     int           `json:"timeout_seconds"`
	GraceSeconds       int           `json:"grace_seconds"`
	FailuresBeforeDown int           `json:"failures_before_down"`
	SuccessesBeforeUp  int           `json:"successes_before_up"`
	LocationsRequired  int           `json:"locations_required"`
	Paused             bool          `json:"paused"`
	State              string        `json:"state"`
	HeartbeatURL       string        `json:"heartbeat_url"`
	AgentKey           string        `json:"agent_key"`
	ChannelIDs         []string      `json:"channel_ids"`
}

func (c *Client) CreateCheck(ctx context.Context, w CheckWrite) (Check, error) {
	var out Check
	return out, c.do(ctx, "POST", "/v1/workspace/checks", w, &out)
}

func (c *Client) GetCheck(ctx context.Context, id string) (Check, error) {
	var out Check
	return out, c.do(ctx, "GET", "/v1/workspace/checks/"+esc(id), nil, &out)
}

func (c *Client) UpdateCheck(ctx context.Context, id string, w CheckWrite) (Check, error) {
	var out Check
	return out, c.do(ctx, "PUT", "/v1/workspace/checks/"+esc(id), w, &out)
}

func (c *Client) DeleteCheck(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/workspace/checks/"+esc(id), nil, nil)
}

// --- channels ------------------------------------------------------------------------------

// ChannelConfig is a channel's settings (secrets are write-only).
type ChannelConfig struct {
	To         []string `json:"to,omitempty"`
	BotToken   string   `json:"bot_token,omitempty"`
	ChatID     string   `json:"chat_id,omitempty"`
	WebhookURL string   `json:"webhook_url,omitempty"`
	URL        string   `json:"url,omitempty"`
}

// ChannelWrite is what a channel is created or changed with.
type ChannelWrite struct {
	Name      string        `json:"name"`
	Kind      string        `json:"kind"`
	AllChecks *bool         `json:"all_checks,omitempty"`
	Enabled   *bool         `json:"enabled,omitempty"`
	Config    ChannelConfig `json:"config"`
}

// Channel is a channel as the server shows it.
type Channel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	AllChecks     bool   `json:"all_checks"`
	Enabled       bool   `json:"enabled"`
	WebhookSecret string `json:"webhook_secret"`
}

func (c *Client) CreateChannel(ctx context.Context, w ChannelWrite) (Channel, error) {
	var out Channel
	return out, c.do(ctx, "POST", "/v1/workspace/channels", w, &out)
}

func (c *Client) GetChannel(ctx context.Context, id string) (Channel, error) {
	var out Channel
	return out, c.do(ctx, "GET", "/v1/workspace/channels/"+esc(id), nil, &out)
}

func (c *Client) UpdateChannel(ctx context.Context, id string, w ChannelWrite) (Channel, error) {
	var out Channel
	return out, c.do(ctx, "PUT", "/v1/workspace/channels/"+esc(id), w, &out)
}

func (c *Client) DeleteChannel(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/workspace/channels/"+esc(id), nil, nil)
}

// --- status pages ----------------------------------------------------------------------------

// StatusPageWrite is what a page is created or changed with.
type StatusPageWrite struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	CustomDomain string `json:"custom_domain,omitempty"`
	Published    *bool  `json:"published,omitempty"`
}

// StatusPage is a page as the server shows it.
type StatusPage struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	CustomDomain   string `json:"custom_domain"`
	DomainToken    string `json:"domain_token"`
	DomainVerified bool   `json:"domain_verified"`
	Published      bool   `json:"published"`
	URL            string `json:"url"`
}

// Component is a check shown on a page.
type Component struct {
	CheckID string `json:"check_id"`
	Name    string `json:"name"`
}

func (c *Client) CreateStatusPage(ctx context.Context, w StatusPageWrite) (StatusPage, error) {
	var out StatusPage
	return out, c.do(ctx, "POST", "/v1/workspace/status-pages", w, &out)
}

func (c *Client) GetStatusPage(ctx context.Context, id string) (StatusPage, error) {
	var out StatusPage
	return out, c.do(ctx, "GET", "/v1/workspace/status-pages/"+esc(id), nil, &out)
}

func (c *Client) UpdateStatusPage(ctx context.Context, id string, w StatusPageWrite) (StatusPage, error) {
	var out StatusPage
	return out, c.do(ctx, "PUT", "/v1/workspace/status-pages/"+esc(id), w, &out)
}

func (c *Client) DeleteStatusPage(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/workspace/status-pages/"+esc(id), nil, nil)
}

// VerifyDomain asks the server to check the page's TXT record now.
func (c *Client) VerifyDomain(ctx context.Context, pageID string) (StatusPage, error) {
	var out StatusPage
	return out, c.do(ctx, "POST", "/v1/workspace/status-pages/"+esc(pageID)+"/verify-domain", nil, &out)
}

func (c *Client) GetComponents(ctx context.Context, pageID string) ([]Component, error) {
	var out []Component
	return out, c.do(ctx, "GET", "/v1/workspace/status-pages/"+esc(pageID)+"/components", nil, &out)
}

func (c *Client) SetComponents(ctx context.Context, pageID string, comps []Component) ([]Component, error) {
	if comps == nil {
		comps = []Component{}
	}
	var out []Component
	return out, c.do(ctx, "PUT", "/v1/workspace/status-pages/"+esc(pageID)+"/components", comps, &out)
}

// --- maintenance -------------------------------------------------------------------------------

// Maintenance is planned work.
type Maintenance struct {
	ID       string    `json:"id,omitempty"`
	PageID   string    `json:"page_id,omitempty"`
	Title    string    `json:"title"`
	Message  string    `json:"message,omitempty"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	CheckIDs []string  `json:"check_ids"`
}

func (c *Client) CreateMaintenance(ctx context.Context, m Maintenance) (Maintenance, error) {
	m.ID = ""
	var out Maintenance
	return out, c.do(ctx, "POST", "/v1/workspace/maintenance", m, &out)
}

// GetMaintenance finds maintenance by id in the list (there is no GET by
// id); ended maintenance drops off the list after 30 days and reads as
// gone.
func (c *Client) GetMaintenance(ctx context.Context, id string) (Maintenance, error) {
	var list []Maintenance
	if err := c.do(ctx, "GET", "/v1/workspace/maintenance", nil, &list); err != nil {
		return Maintenance{}, err
	}
	for _, m := range list {
		if m.ID == id {
			return m, nil
		}
	}
	return Maintenance{}, &Error{Status: http.StatusNotFound, Code: "not_found", Message: "no such maintenance (or it ended more than 30 days ago)"}
}

func (c *Client) DeleteMaintenance(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/workspace/maintenance/"+esc(id), nil, nil)
}
