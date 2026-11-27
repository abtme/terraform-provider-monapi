package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-monapi/internal/client"
)

var _ datasource.DataSource = &workspaceDataSource{}

func newWorkspaceDataSource() datasource.DataSource { return &workspaceDataSource{} }

type workspaceDataSource struct{ c *client.Client }

type workspaceModel struct {
	ID   types.String `tfsdk:"id"`
	Slug types.String `tfsdk:"slug"`
	Name types.String `tfsdk:"name"`
	Role types.String `tfsdk:"role"`
}

func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The workspace the provider's token belongs to, and what the token may do there.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true},
			"slug": schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{Computed: true},
			"role": schema.StringAttribute{Computed: true, Description: "viewer, editor or owner."},
		},
	}
}

func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *workspaceDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	w, err := d.c.Workspace(ctx)
	if err != nil {
		apiError(&resp.Diagnostics, "reading the workspace", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &workspaceModel{ID: types.StringValue(w.ID), Slug: types.StringValue(w.Slug),
		Name: types.StringValue(w.Name), Role: types.StringValue(w.Role)})...)
}
