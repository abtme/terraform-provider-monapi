package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-monapi/internal/client"
)

var (
	_ resource.Resource                   = &checkResource{}
	_ resource.ResourceWithImportState    = &checkResource{}
	_ resource.ResourceWithValidateConfig = &checkResource{}
)

func newCheckResource() resource.Resource { return &checkResource{} }

type checkResource struct{ c *client.Client }

type checkModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Kind               types.String `tfsdk:"kind"`
	Target             types.String `tfsdk:"target"`
	Port               types.Int64  `tfsdk:"port"`
	IntervalSeconds    types.Int64  `tfsdk:"interval_seconds"`
	TimeoutSeconds     types.Int64  `tfsdk:"timeout_seconds"`
	GraceSeconds       types.Int64  `tfsdk:"grace_seconds"`
	FailuresBeforeDown types.Int64  `tfsdk:"failures_before_down"`
	SuccessesBeforeUp  types.Int64  `tfsdk:"successes_before_up"`
	LocationsRequired  types.Int64  `tfsdk:"locations_required"`
	Paused             types.Bool   `tfsdk:"paused"`
	ChannelIDs         types.Set    `tfsdk:"channel_ids"`
	ExpectStatus       types.Int64  `tfsdk:"expect_status"`
	ExpectText         types.String `tfsdk:"expect_text"`
	FollowRedirects    types.Bool   `tfsdk:"follow_redirects"`
	SkipTLSVerify      types.Bool   `tfsdk:"skip_tls_verify"`
	RequireSTARTTLS    types.Bool   `tfsdk:"require_starttls"`
	RecordType         types.String `tfsdk:"record_type"`
	ExpectValue        types.String `tfsdk:"expect_value"`
	Resolver           types.String `tfsdk:"resolver"`
	ExpireWarnDays     types.Int64  `tfsdk:"expire_warn_days"`
	ServerName         types.String `tfsdk:"server_name"`
	Lists              types.List   `tfsdk:"lists"`
	Services           types.List   `tfsdk:"services"`
	Processes          types.List   `tfsdk:"processes"`
	Ports              types.List   `tfsdk:"ports"`
	DiskPercent        types.Int64  `tfsdk:"disk_percent"`
	MemPercent         types.Int64  `tfsdk:"mem_percent"`
	SMART              types.Bool   `tfsdk:"smart"`
	State              types.String `tfsdk:"state"`
	HeartbeatURL       types.String `tfsdk:"heartbeat_url"`
	AgentKey           types.String `tfsdk:"agent_key"`
}

var kinds = []string{"http", "tcp", "ping", "smtp", "dns", "heartbeat", "tls", "blacklist", "domain", "agent"}

// kindAttrs lists the kind-specific attributes each kind takes; setting one
// another kind ignores is an error (the server would drop it, and
// Terraform would then see a different value from the one configured).
var kindAttrs = map[string][]string{
	"http":      {"expect_status", "expect_text", "follow_redirects", "skip_tls_verify"},
	"tcp":       {"port"},
	"smtp":      {"port", "require_starttls"},
	"ping":      {},
	"dns":       {"record_type", "expect_value", "resolver"},
	"heartbeat": {},
	"tls":       {"port", "server_name", "expire_warn_days"},
	"domain":    {"expire_warn_days"},
	"blacklist": {"lists"},
	"agent":     {"services", "processes", "ports", "disk_percent", "mem_percent", "smart"},
}

var allKindAttrs = []string{"port", "expect_status", "expect_text", "follow_redirects", "skip_tls_verify", "require_starttls",
	"record_type", "expect_value", "resolver", "expire_warn_days", "server_name", "lists", "services", "processes", "ports",
	"disk_percent", "mem_percent", "smart"}

func (r *checkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

func optInt(desc string, v ...validator.Int64) schema.Int64Attribute {
	return schema.Int64Attribute{Optional: true, Computed: true, Description: desc, Validators: v}
}

func optStr(desc string, v ...validator.String) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Computed: true, Description: desc, Validators: v}
}

func optBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Computed: true, Description: desc}
}

var zoneRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`)

func (r *checkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A check: something monapi watches (a website, port, mail server, DNS answer, certificate, " +
			"domain, blacklist listing, heartbeat or server agent). Unset optional values take the server's defaults; " +
			"to go back to a default after setting a value, set the default explicitly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, Description: "Shown in notifications and the app (1 to 100 characters).",
				Validators: []validator.String{stringvalidator.LengthBetween(1, 100)}},
			"kind": schema.StringAttribute{Required: true, Description: "One of " + strings.Join(kinds, ", ") + ". Changing it makes a new check.",
				Validators:    []validator.String{stringvalidator.OneOf(kinds...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"target": schema.StringAttribute{Optional: true, Computed: true,
				Description: "A URL (http), host name or address (tcp, ping, smtp, tls, blacklist), name (dns) or domain (domain). Not used by heartbeat and agent checks."},
			"port": optInt("tcp (required), smtp (default 25), tls (default 443).", int64validator.Between(1, 65535)),
			"interval_seconds": optInt("How often to check (minimum and default depend on the kind; up to 86400). " +
				"For a heartbeat, how often the job should check in."),
			"timeout_seconds":      optInt("1 to 60, shorter than the interval.", int64validator.Between(1, 60)),
			"grace_seconds":        optInt("Heartbeat: how late a check-in may be.", int64validator.Between(0, 86400)),
			"failures_before_down": optInt("Failures in a row (per location) before a location counts it down; 1 to 10.", int64validator.Between(1, 10)),
			"successes_before_up":  optInt("Successes in a row before it counts as up again; 1 to 10.", int64validator.Between(1, 10)),
			"locations_required":   optInt("How many probe locations must agree it is down; 1 to 20.", int64validator.Between(1, 20)),
			"paused": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "A paused check is not run and notifies nobody."},
			"channel_ids": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				Description: "Channels to notify besides those set to all checks."},
			"expect_status":    optInt("http: the status that counts as up (default any 2xx or 3xx).", int64validator.Between(100, 599)),
			"expect_text":      optStr("http: text that must be in the page.", stringvalidator.LengthAtMost(200)),
			"follow_redirects": optBool("http: follow redirects (default false: a redirect is the answer)."),
			"skip_tls_verify":  optBool("http: accept any certificate."),
			"require_starttls": optBool("smtp: the server must offer STARTTLS."),
			"record_type":      optStr("dns: A (default), AAAA, CNAME, MX, TXT or NS.", stringvalidator.OneOf("A", "AAAA", "CNAME", "MX", "TXT", "NS")),
			"expect_value":     optStr("dns: one answer must equal this."),
			"resolver":         optStr("dns: the resolver to ask, host:port (default the probe's own)."),
			"expire_warn_days": optInt("tls (default 14) and domain (default 30): fail this many days before expiry.", int64validator.Between(1, 365)),
			"server_name":      optStr("tls: the name the certificate must be for (default the target)."),
			"lists": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				Description: "blacklist: DNSBL zones to ask (default a well-known set), lower case.",
				Validators: []validator.List{listvalidator.SizeAtMost(50), listvalidator.ValueStringsAre(stringvalidator.RegexMatches(zoneRe,
					"must be a DNS zone in lower case, such as zen.spamhaus.org"))}},
			"services": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				Description: "agent: systemd units that must be active.", Validators: []validator.List{listvalidator.SizeAtMost(50)}},
			"processes": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				Description: "agent: processes that must be running (as ps shows them, or the program or script file name).",
				Validators:  []validator.List{listvalidator.SizeAtMost(50)}},
			"ports": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.Int64Type,
				Description: "agent: TCP ports something must listen on.",
				Validators:  []validator.List{listvalidator.SizeAtMost(50), listvalidator.ValueInt64sAre(int64validator.Between(1, 65535))}},
			"disk_percent": optInt("agent: a filesystem fuller than this is a problem (default 90).", int64validator.Between(1, 100)),
			"mem_percent":  optInt("agent: memory use above this is a problem (default 95).", int64validator.Between(1, 100)),
			"smart":        optBool("agent: ask SMART health (the agent needs smartctl through sudo)."),
			"state":        schema.StringAttribute{Computed: true, Description: "up, down, unknown or paused, when last read."},
			"heartbeat_url": schema.StringAttribute{Computed: true, Description: "heartbeat: the URL the job calls.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"agent_key": schema.StringAttribute{Computed: true, Sensitive: true, Description: "agent: the key the agent reports with.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *checkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *checkResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m checkModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || m.Kind.IsNull() || m.Kind.IsUnknown() {
		return
	}
	k := m.Kind.ValueString()
	set := map[string]bool{
		"port": !m.Port.IsNull(), "expect_status": !m.ExpectStatus.IsNull(), "expect_text": !m.ExpectText.IsNull(),
		"follow_redirects": !m.FollowRedirects.IsNull(), "skip_tls_verify": !m.SkipTLSVerify.IsNull(),
		"require_starttls": !m.RequireSTARTTLS.IsNull(), "record_type": !m.RecordType.IsNull(),
		"expect_value": !m.ExpectValue.IsNull(), "resolver": !m.Resolver.IsNull(), "expire_warn_days": !m.ExpireWarnDays.IsNull(),
		"server_name": !m.ServerName.IsNull(), "lists": !m.Lists.IsNull(), "services": !m.Services.IsNull(),
		"processes": !m.Processes.IsNull(), "ports": !m.Ports.IsNull(), "disk_percent": !m.DiskPercent.IsNull(),
		"mem_percent": !m.MemPercent.IsNull(), "smart": !m.SMART.IsNull(),
	}
	allowed := map[string]bool{}
	for _, a := range kindAttrs[k] {
		allowed[a] = true
	}
	for _, a := range allKindAttrs {
		if set[a] && !allowed[a] {
			resp.Diagnostics.AddAttributeError(path.Root(a), "Not used by this kind", fmt.Sprintf("%s is not used by %s checks.", a, k))
		}
	}
	noTarget := k == "heartbeat" || k == "agent"
	switch {
	case noTarget && !m.Target.IsNull():
		resp.Diagnostics.AddAttributeError(path.Root("target"), "Not used by this kind", k+" checks have no target.")
	case !noTarget && m.Target.IsNull():
		resp.Diagnostics.AddAttributeError(path.Root("target"), "Missing target", k+" checks need a target.")
	}
	if k == "tcp" && m.Port.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("port"), "Missing port", "tcp checks need a port.")
	}
}

func i64(v types.Int64) int {
	if v.IsNull() || v.IsUnknown() {
		return 0
	}
	return int(v.ValueInt64())
}

func str(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

func boo(v types.Bool) bool { return !v.IsNull() && !v.IsUnknown() && v.ValueBool() }

func (m *checkModel) write(ctx context.Context, diags *diag.Diagnostics) client.CheckWrite {
	w := client.CheckWrite{
		Name: m.Name.ValueString(), Kind: m.Kind.ValueString(), Target: str(m.Target), Port: i64(m.Port),
		IntervalSeconds: i64(m.IntervalSeconds), TimeoutSeconds: i64(m.TimeoutSeconds), GraceSeconds: i64(m.GraceSeconds),
		FailuresBeforeDown: i64(m.FailuresBeforeDown), SuccessesBeforeUp: i64(m.SuccessesBeforeUp),
		LocationsRequired: i64(m.LocationsRequired), Paused: boo(m.Paused),
		Settings: client.CheckSettings{
			ExpectStatus: i64(m.ExpectStatus), ExpectText: str(m.ExpectText), FollowRedirects: boo(m.FollowRedirects),
			SkipTLSVerify: boo(m.SkipTLSVerify), RequireSTARTTLS: boo(m.RequireSTARTTLS), RecordType: str(m.RecordType),
			ExpectValue: str(m.ExpectValue), Resolver: str(m.Resolver), ExpireWarnDays: i64(m.ExpireWarnDays),
			ServerName: str(m.ServerName), Lists: stringList(ctx, m.Lists, diags), Services: stringList(ctx, m.Services, diags),
			Processes: stringList(ctx, m.Processes, diags), Ports: ints(ctx, m.Ports, diags),
			DiskPercent: i64(m.DiskPercent), MemPercent: i64(m.MemPercent), SMART: boo(m.SMART),
		},
	}
	if !m.ChannelIDs.IsNull() && !m.ChannelIDs.IsUnknown() {
		ids := stringSet(ctx, m.ChannelIDs, diags)
		w.ChannelIDs = &ids
	}
	return w
}

// sameTarget: the server trims targets and lowers domain names; keep what
// was written when it means the same.
func sameTarget(a, b string) bool {
	n := func(s string) string { return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), ".")) }
	return n(a) == n(b)
}

func (m *checkModel) read(c client.Check) {
	m.ID = types.StringValue(c.ID)
	m.Name = types.StringValue(c.Name)
	m.Kind = types.StringValue(c.Kind)
	if m.Target.IsNull() || m.Target.IsUnknown() || !sameTarget(m.Target.ValueString(), c.Target) {
		if c.Target == "" && (c.Kind == "heartbeat" || c.Kind == "agent") {
			m.Target = types.StringNull()
		} else {
			m.Target = types.StringValue(c.Target)
		}
	}
	m.Port = types.Int64Value(int64(c.Port))
	m.IntervalSeconds = types.Int64Value(int64(c.IntervalSeconds))
	m.TimeoutSeconds = types.Int64Value(int64(c.TimeoutSeconds))
	m.GraceSeconds = types.Int64Value(int64(c.GraceSeconds))
	m.FailuresBeforeDown = types.Int64Value(int64(c.FailuresBeforeDown))
	m.SuccessesBeforeUp = types.Int64Value(int64(c.SuccessesBeforeUp))
	m.LocationsRequired = types.Int64Value(int64(c.LocationsRequired))
	m.Paused = types.BoolValue(c.Paused)
	ids := append([]string{}, c.ChannelIDs...)
	sort.Strings(ids)
	m.ChannelIDs = setOfStrings(ids)
	s := c.Settings
	m.ExpectStatus = types.Int64Value(int64(s.ExpectStatus))
	m.ExpectText = types.StringValue(s.ExpectText)
	m.FollowRedirects = types.BoolValue(s.FollowRedirects)
	m.SkipTLSVerify = types.BoolValue(s.SkipTLSVerify)
	m.RequireSTARTTLS = types.BoolValue(s.RequireSTARTTLS)
	m.RecordType = types.StringValue(s.RecordType)
	m.ExpectValue = types.StringValue(s.ExpectValue)
	m.Resolver = types.StringValue(s.Resolver)
	m.ExpireWarnDays = types.Int64Value(int64(s.ExpireWarnDays))
	m.ServerName = types.StringValue(s.ServerName)
	m.Lists = listOfStrings(orEmpty(s.Lists))
	m.Services = listOfStrings(orEmpty(s.Services))
	m.Processes = listOfStrings(orEmpty(s.Processes))
	m.Ports = listOfInts(s.Ports)
	m.DiskPercent = types.Int64Value(int64(s.DiskPercent))
	m.MemPercent = types.Int64Value(int64(s.MemPercent))
	m.SMART = types.BoolValue(s.SMART)
	m.State = types.StringValue(c.State)
	m.HeartbeatURL = types.StringValue(c.HeartbeatURL)
	m.AgentKey = types.StringValue(c.AgentKey)
}

func orEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func (r *checkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m checkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	w := m.write(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.CreateCheck(ctx, w)
	if err != nil {
		apiError(&resp.Diagnostics, "creating the check", err)
		return
	}
	m.read(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *checkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m checkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.GetCheck(ctx, m.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "reading the check", err)
		return
	}
	m.read(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *checkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, prior checkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	w := m.write(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.UpdateCheck(ctx, prior.ID.ValueString(), w)
	if err != nil {
		apiError(&resp.Diagnostics, "changing the check", err)
		return
	}
	m.read(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *checkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m checkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.DeleteCheck(ctx, m.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiError(&resp.Diagnostics, "removing the check", err)
	}
}

func (r *checkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
