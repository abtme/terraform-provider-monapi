package provider

import (
	"context"
	"regexp"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-monapi/internal/client"
)

var (
	slugRe          = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)
	domainOrEmptyRe = regexp.MustCompile(`^(([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,})?$`)
	uuidRe          = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// --- maintenance ------------------------------------------------------------------------

var (
	_ resource.Resource                = &maintenanceResource{}
	_ resource.ResourceWithImportState = &maintenanceResource{}
)

func newMaintenanceResource() resource.Resource { return &maintenanceResource{} }

type maintenanceResource struct{ c *client.Client }

type maintenanceModel struct {
	ID       types.String `tfsdk:"id"`
	Title    types.String `tfsdk:"title"`
	Message  types.String `tfsdk:"message"`
	PageID   types.String `tfsdk:"page_id"`
	StartsAt types.String `tfsdk:"starts_at"`
	EndsAt   types.String `tfsdk:"ends_at"`
	CheckIDs types.Set    `tfsdk:"check_ids"`
}

func (r *maintenanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_maintenance"
}

func replaceStr() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func (r *maintenanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	rfc := stringvalidator.RegexMatches(regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d(:\d\d(\.\d+)?)?(Z|[+-]\d\d:\d\d)$`),
		"must be an RFC 3339 time, such as 2026-12-01T02:00:00Z")
	resp.Schema = schema.Schema{
		Description: "Planned maintenance: while it runs its checks notify nobody (what happens is still recorded), " +
			"and a status page can show it in advance. Maintenance cannot be changed, only replaced, so any change " +
			"cancels it and plans it again. Maintenance that ended more than 30 days ago is no longer listed by the " +
			"server; it stays in the state until removed from the configuration.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"title": schema.StringAttribute{Required: true, PlanModifiers: replaceStr(), Validators: []validator.String{stringvalidator.LengthBetween(1, 200)}},
			"message": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: replaceStr(),
				Description: "For the status page.", Validators: []validator.String{stringvalidator.LengthAtMost(5000)}},
			"page_id": schema.StringAttribute{Optional: true, PlanModifiers: replaceStr(),
				Description: "The status page to show it on (optional)."},
			"starts_at": schema.StringAttribute{Required: true, PlanModifiers: replaceStr(), Validators: []validator.String{rfc}},
			"ends_at": schema.StringAttribute{Required: true, PlanModifiers: replaceStr(), Validators: []validator.String{rfc},
				Description: "After starts_at, at most 30 days later."},
			"check_ids": schema.SetAttribute{Required: true, ElementType: types.StringType,
				PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
				Validators:    []validator.Set{setvalidator.SizeAtLeast(1)}},
		},
	}
}

func (r *maintenanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func parseTime(diags *diag.Diagnostics, attr string, v types.String) time.Time {
	t, err := time.Parse(time.RFC3339, v.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root(attr), "Not a time", err.Error())
	}
	return t
}

// keepTime keeps the configured spelling when it is the same instant.
func keepTime(cur types.String, t time.Time) types.String {
	if !cur.IsNull() && !cur.IsUnknown() {
		if c, err := time.Parse(time.RFC3339, cur.ValueString()); err == nil && c.Equal(t) {
			return cur
		}
	}
	return types.StringValue(t.UTC().Format(time.RFC3339))
}

func (m *maintenanceModel) read(x client.Maintenance) {
	m.ID = types.StringValue(x.ID)
	m.Title = types.StringValue(x.Title)
	m.Message = types.StringValue(x.Message)
	if x.PageID == "" {
		m.PageID = types.StringNull()
	} else {
		m.PageID = types.StringValue(x.PageID)
	}
	m.StartsAt = keepTime(m.StartsAt, x.StartsAt)
	m.EndsAt = keepTime(m.EndsAt, x.EndsAt)
	ids := append([]string{}, x.CheckIDs...)
	sort.Strings(ids)
	m.CheckIDs = setOfStrings(ids)
}

func (r *maintenanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m maintenanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	x := client.Maintenance{Title: m.Title.ValueString(), Message: str(m.Message), PageID: str(m.PageID),
		StartsAt: parseTime(&resp.Diagnostics, "starts_at", m.StartsAt), EndsAt: parseTime(&resp.Diagnostics, "ends_at", m.EndsAt),
		CheckIDs: stringSet(ctx, m.CheckIDs, &resp.Diagnostics)}
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.c.CreateMaintenance(ctx, x)
	if err != nil {
		apiError(&resp.Diagnostics, "planning the maintenance", err)
		return
	}
	m.read(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *maintenanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m maintenanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	x, err := r.c.GetMaintenance(ctx, m.ID.ValueString())
	if client.IsNotFound(err) {
		// Long over: the server stops listing it. Keep it rather than plan it
		// again; someone cancelling it early is the other way it goes away.
		if end, perr := time.Parse(time.RFC3339, m.EndsAt.ValueString()); perr == nil && end.Before(time.Now().Add(-29*24*time.Hour)) {
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "reading the maintenance", err)
		return
	}
	m.read(x)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *maintenanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every attribute forces replacement; nothing to do.
	var m maintenanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *maintenanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m maintenanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.DeleteMaintenance(ctx, m.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiError(&resp.Diagnostics, "cancelling the maintenance", err)
	}
}

func (r *maintenanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}

// --- domain verification ------------------------------------------------------------------

var _ resource.Resource = &domainVerificationResource{}

func newDomainVerificationResource() resource.Resource { return &domainVerificationResource{} }

type domainVerificationResource struct{ c *client.Client }

type domainVerificationModel struct {
	ID       types.String `tfsdk:"id"`
	PageID   types.String `tfsdk:"page_id"`
	Domain   types.String `tfsdk:"custom_domain"`
	Verified types.Bool   `tfsdk:"verified"`
}

func (r *domainVerificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_page_domain_verification"
}

func (r *domainVerificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Verifies a status page's custom domain: monapi looks for the page's domain_token in a TXT record " +
			"at _monapi.<custom_domain>. Make the record first (depends_on it, or reference it). Creating this resource " +
			"fails while the record is not visible yet; apply again once DNS has it. A changed custom_domain makes a new " +
			"verification. Removing this resource does not undo the verification.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"page_id": schema.StringAttribute{Required: true, PlanModifiers: replaceStr(),
				Validators: []validator.String{stringvalidator.RegexMatches(uuidRe, "must be a status page id")}},
			"custom_domain": schema.StringAttribute{Required: true, PlanModifiers: replaceStr(),
				Description: "The page's custom_domain (pass it through so a change verifies again)."},
			"verified": schema.BoolAttribute{Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *domainVerificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *domainVerificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m domainVerificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.GetStatusPage(ctx, m.PageID.ValueString())
	if err == nil && !domainVerified(p, m.Domain.ValueString()) {
		if p.CustomDomain != m.Domain.ValueString() {
			resp.Diagnostics.AddAttributeError(path.Root("custom_domain"), "Not the page's domain",
				"The status page's custom_domain is "+quote(p.CustomDomain)+", not "+quote(m.Domain.ValueString())+".")
			return
		}
		p, err = r.c.VerifyDomain(ctx, p.ID)
	}
	if err != nil {
		apiError(&resp.Diagnostics, "verifying the domain", err)
		return
	}
	m.ID = types.StringValue(p.ID + "/" + p.CustomDomain)
	m.Verified = types.BoolValue(true)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *domainVerificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m domainVerificationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.GetStatusPage(ctx, m.PageID.ValueString())
	if client.IsNotFound(err) || (err == nil && !domainVerified(p, m.Domain.ValueString())) {
		resp.State.RemoveResource(ctx) // the page went, or its domain changed: verify again
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "reading the status page", err)
	}
}

func (r *domainVerificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m domainVerificationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *domainVerificationResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func quote(s string) string {
	if s == "" {
		return "empty"
	}
	return `"` + s + `"`
}
