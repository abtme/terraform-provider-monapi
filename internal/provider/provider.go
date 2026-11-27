// Package provider implements the monapi Terraform provider.
package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/abtme/terraform-provider-monapi/internal/client"
)

var _ provider.Provider = &monapiProvider{}

type monapiProvider struct{ version string }

type providerModel struct {
	URL   types.String `tfsdk:"url"`
	Token types.String `tfsdk:"token"`
}

// New returns the provider.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &monapiProvider{version: version} }
}

func (p *monapiProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "monapi"
	resp.Version = p.version
}

func (p *monapiProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a monapi workspace: checks, notification channels, status pages and maintenance. " +
			"Each API token belongs to one workspace; use one provider block (or alias) per workspace.",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Optional:    true,
				Description: "The monapi server, such as https://monitor.example.com. Or MONAPI_URL.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "A workspace API token (mnp_...), made under the workspace's Settings. Or MONAPI_TOKEN. Use an editor token (owner to manage channels).",
			},
		},
	}
}

func (p *monapiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	url, token := os.Getenv("MONAPI_URL"), os.Getenv("MONAPI_TOKEN")
	if !cfg.URL.IsNull() && !cfg.URL.IsUnknown() {
		url = cfg.URL.ValueString()
	}
	if !cfg.Token.IsNull() && !cfg.Token.IsUnknown() {
		token = cfg.Token.ValueString()
	}
	if url == "" || token == "" {
		resp.Diagnostics.AddError("Missing monapi settings", "Set url and token in the provider block, or MONAPI_URL and MONAPI_TOKEN.")
		return
	}
	c, err := client.New(url, token, client.WithUserAgent("terraform-provider-monapi/"+p.version),
		client.WithDebug(func(f string, a ...any) { tflog.Debug(ctx, "monapi: "+fmt.Sprintf(f, a...)) }))
	if err != nil {
		resp.Diagnostics.AddError("Bad monapi settings", err.Error())
		return
	}
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *monapiProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{newCheckResource, newChannelResource, newStatusPageResource, newDomainVerificationResource, newMaintenanceResource}
}

func (p *monapiProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{newWorkspaceDataSource}
}
