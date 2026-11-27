package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-monapi/internal/client"
)

var (
	_ resource.Resource                = &statusPageResource{}
	_ resource.ResourceWithImportState = &statusPageResource{}
)

func newStatusPageResource() resource.Resource { return &statusPageResource{} }

type statusPageResource struct{ c *client.Client }

type statusPageModel struct {
	ID             types.String `tfsdk:"id"`
	Slug           types.String `tfsdk:"slug"`
	Title          types.String `tfsdk:"title"`
	Description    types.String `tfsdk:"description"`
	CustomDomain   types.String `tfsdk:"custom_domain"`
	Published      types.Bool   `tfsdk:"published"`
	Components     types.List   `tfsdk:"components"`
	DomainToken    types.String `tfsdk:"domain_token"`
	DomainVerified types.Bool   `tfsdk:"domain_verified"`
	URL            types.String `tfsdk:"url"`
}

type componentModel struct {
	CheckID types.String `tfsdk:"check_id"`
	Name    types.String `tfsdk:"name"`
}

var componentType = types.ObjectType{AttrTypes: map[string]attr.Type{"check_id": types.StringType, "name": types.StringType}}

func (r *statusPageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_page"
}

func (r *statusPageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A public status page: chosen checks under public names, their uptime, incidents and planned " +
			"maintenance. Incidents are posted by people (in the app or API), not managed here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"slug": schema.StringAttribute{Required: true, Description: "The address: /s/<slug>. 3 to 50 of a-z, 0-9 and -.",
				Validators: []validator.String{stringvalidator.RegexMatches(slugRe, "must be 3 to 50 of a-z, 0-9 and - (not at either end)")}},
			"title": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 100)}},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Validators: []validator.String{stringvalidator.LengthAtMost(1000)}},
			"custom_domain": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Description: "Serve the page on this name too, once verified (see monapi_status_page_domain_verification). " +
					"Point it at the monapi server (CNAME) and give that server a certificate for it.",
				Validators: []validator.String{stringvalidator.RegexMatches(domainOrEmptyRe, "must be a host name in lower case")}},
			"published": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				Description: "An unpublished page is not shown to anyone."},
			"components": schema.ListNestedAttribute{Optional: true, Computed: true,
				Description: "The checks shown, in this order, each under a public name (the check's own name and target are never shown).",
				Validators:  []validator.List{listvalidator.SizeAtMost(100)},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"check_id": schema.StringAttribute{Required: true},
					"name":     schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 100)}},
				}}},
			"domain_token": schema.StringAttribute{Computed: true,
				Description:   "Put this in a TXT record at _monapi.<custom_domain> to verify the domain.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"domain_verified": schema.BoolAttribute{Computed: true, Description: "Whether custom_domain has been verified (a changed domain must be verified again)."},
			"url":             schema.StringAttribute{Computed: true, Description: "The page's address on the monapi server."},
		},
	}
}

func (r *statusPageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (m *statusPageModel) toWrite() client.StatusPageWrite {
	p := m.Published.ValueBool()
	return client.StatusPageWrite{Slug: m.Slug.ValueString(), Title: m.Title.ValueString(), Description: str(m.Description),
		CustomDomain: str(m.CustomDomain), Published: &p}
}

func (m *statusPageModel) components(ctx context.Context, diags *diag.Diagnostics) ([]client.Component, bool) {
	if m.Components.IsNull() || m.Components.IsUnknown() {
		return nil, false
	}
	var cs []componentModel
	diags.Append(m.Components.ElementsAs(ctx, &cs, false)...)
	out := make([]client.Component, len(cs))
	for i, c := range cs {
		out[i] = client.Component{CheckID: c.CheckID.ValueString(), Name: c.Name.ValueString()}
	}
	return out, true
}

func (m *statusPageModel) read(p client.StatusPage, comps []client.Component) {
	m.ID = types.StringValue(p.ID)
	m.Slug = types.StringValue(p.Slug)
	m.Title = types.StringValue(p.Title)
	m.Description = types.StringValue(p.Description)
	m.CustomDomain = types.StringValue(p.CustomDomain)
	m.Published = types.BoolValue(p.Published)
	m.DomainToken = types.StringValue(p.DomainToken)
	m.DomainVerified = types.BoolValue(p.DomainVerified)
	m.URL = types.StringValue(p.URL)
	elems := make([]attr.Value, len(comps))
	for i, c := range comps {
		elems[i] = types.ObjectValueMust(componentType.AttrTypes, map[string]attr.Value{
			"check_id": types.StringValue(c.CheckID), "name": types.StringValue(c.Name)})
	}
	m.Components = types.ListValueMust(componentType, elems)
}

func (r *statusPageResource) save(ctx context.Context, m *statusPageModel, p client.StatusPage, diags *diag.Diagnostics) {
	comps, set := m.components(ctx, diags)
	var err error
	if set {
		comps, err = r.c.SetComponents(ctx, p.ID, comps)
	} else {
		comps, err = r.c.GetComponents(ctx, p.ID)
	}
	if err != nil {
		apiError(diags, "the status page's components", err)
		return
	}
	m.read(p, comps)
}

func (r *statusPageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m statusPageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.CreateStatusPage(ctx, m.toWrite())
	if err != nil {
		apiError(&resp.Diagnostics, "creating the status page", err)
		return
	}
	// Saved before the components, so a failure there leaves a page Terraform knows about.
	m.ID = types.StringValue(p.ID)
	r.save(ctx, &m, p, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		m.read(p, nil)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *statusPageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m statusPageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.GetStatusPage(ctx, m.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "reading the status page", err)
		return
	}
	comps, err := r.c.GetComponents(ctx, p.ID)
	if err != nil {
		apiError(&resp.Diagnostics, "reading the status page's components", err)
		return
	}
	m.read(p, comps)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *statusPageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, prior statusPageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.c.UpdateStatusPage(ctx, prior.ID.ValueString(), m.toWrite())
	if err != nil {
		apiError(&resp.Diagnostics, "changing the status page", err)
		return
	}
	r.save(ctx, &m, p, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}

func (r *statusPageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m statusPageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.DeleteStatusPage(ctx, m.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiError(&resp.Diagnostics, "removing the status page", err)
	}
}

func (r *statusPageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}

// domainVerified reports whether custom domain d is the page's and verified.
func domainVerified(p client.StatusPage, d string) bool {
	return p.DomainVerified && strings.EqualFold(p.CustomDomain, d)
}
