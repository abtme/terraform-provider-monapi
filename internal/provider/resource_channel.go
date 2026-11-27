package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource                   = &channelResource{}
	_ resource.ResourceWithImportState    = &channelResource{}
	_ resource.ResourceWithValidateConfig = &channelResource{}
)

func newChannelResource() resource.Resource { return &channelResource{} }

type channelResource struct{ c *client.Client }

type channelModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Kind          types.String `tfsdk:"kind"`
	AllChecks     types.Bool   `tfsdk:"all_checks"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	To            types.List   `tfsdk:"to"`
	BotToken      types.String `tfsdk:"bot_token"`
	ChatID        types.String `tfsdk:"chat_id"`
	WebhookURL    types.String `tfsdk:"webhook_url"`
	URL           types.String `tfsdk:"url"`
	Summary       types.String `tfsdk:"summary"`
	WebhookSecret types.String `tfsdk:"webhook_secret"`
}

// channelAttrs: the settings each kind needs (all required for it).
var channelAttrs = map[string][]string{
	"email":    {"to"},
	"telegram": {"bot_token", "chat_id"},
	"slack":    {"webhook_url"},
	"teams":    {"webhook_url"},
	"webhook":  {"url"},
}

func (r *channelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_channel"
}

func (r *channelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A notification channel: where state changes are sent. Needs an owner token. The server keeps the " +
			"settings sealed and never returns them, so changes made outside Terraform to to, bot_token, chat_id, " +
			"webhook_url or url are not seen; an import cannot fill them either (set them in the configuration and apply).",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 100)}},
			"kind": schema.StringAttribute{Required: true, Description: "email, telegram, slack, teams or webhook. Changing it makes a new channel.",
				Validators:    []validator.String{stringvalidator.OneOf("email", "telegram", "slack", "teams", "webhook")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"all_checks": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				Description: "Send for every check (default). If false, only for checks that list this channel in channel_ids."},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"to": schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "email: up to 10 addresses.",
				Validators: []validator.List{listvalidator.SizeBetween(1, 10)}},
			"bot_token":   schema.StringAttribute{Optional: true, Sensitive: true, Description: "telegram: the bot's token from @BotFather."},
			"chat_id":     schema.StringAttribute{Optional: true, Description: "telegram: a chat or channel id (-100...) or @channelname."},
			"webhook_url": schema.StringAttribute{Optional: true, Sensitive: true, Description: "slack: an incoming webhook URL; teams: a Workflows \"post to a channel\" URL."},
			"url":         schema.StringAttribute{Optional: true, Description: "webhook: where to POST (https)."},
			"summary":     schema.StringAttribute{Computed: true, Description: "What the server shows for the settings (secrets hidden)."},
			"webhook_secret": schema.StringAttribute{Computed: true, Sensitive: true,
				Description:   "webhook: the key the X-Monapi-Signature HMAC is made with.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *channelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *channelResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m channelModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || m.Kind.IsNull() || m.Kind.IsUnknown() {
		return
	}
	k := m.Kind.ValueString()
	set := map[string]bool{"to": !m.To.IsNull(), "bot_token": !m.BotToken.IsNull(), "chat_id": !m.ChatID.IsNull(),
		"webhook_url": !m.WebhookURL.IsNull(), "url": !m.URL.IsNull()}
	need := map[string]bool{}
	for _, a := range channelAttrs[k] {
		need[a] = true
		if !set[a] {
			resp.Diagnostics.AddAttributeError(path.Root(a), "Missing setting", fmt.Sprintf("%s channels need %s.", k, a))
		}
	}
	for _, a := range []string{"to", "bot_token", "chat_id", "webhook_url", "url"} {
		if set[a] && !need[a] {
			resp.Diagnostics.AddAttributeError(path.Root(a), "Not used by this kind", fmt.Sprintf("%s is not used by %s channels.", a, k))
		}
	}
}

func (m *channelModel) toWrite(ctx context.Context, diags *diag.Diagnostics) client.ChannelWrite {
	all, en := m.AllChecks.ValueBool(), m.Enabled.ValueBool()
	return client.ChannelWrite{Name: m.Name.ValueString(), Kind: m.Kind.ValueString(), AllChecks: &all, Enabled: &en,
		Config: client.ChannelConfig{To: stringList(ctx, m.To, diags), BotToken: str(m.BotToken), ChatID: str(m.ChatID),
			WebhookURL: str(m.WebhookURL), URL: str(m.URL)}}
}

// read takes what the server shows; the sealed settings stay as configured.
func (m *channelModel) read(c client.Channel) {
	m.ID = types.StringValue(c.ID)
	m.Name = types.StringValue(c.Name)
	m.Kind = types.StringValue(c.Kind)
	m.AllChecks = types.BoolValue(c.AllChecks)
	m.Enabled = types.BoolValue(c.Enabled)
	m.Summary = types.StringValue(c.Summary)
	m.WebhookSecret = types.StringValue(c.WebhookSecret) // webhook channels only; the server shows it on every read
}

func (r *channelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m channelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	w := m.toWrite(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.CreateChannel(ctx, w)
	if err != nil {
		apiError(&resp.Diagnostics, "creating the channel", err)
		return
	}
	m.read(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *channelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m channelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.GetChannel(ctx, m.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiError(&resp.Diagnostics, "reading the channel", err)
		return
	}
	m.read(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *channelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, prior channelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	w := m.toWrite(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.c.UpdateChannel(ctx, prior.ID.ValueString(), w)
	if err != nil {
		apiError(&resp.Diagnostics, "changing the channel", err)
		return
	}
	m.read(c)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *channelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m channelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.DeleteChannel(ctx, m.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiError(&resp.Diagnostics, "removing the channel", err)
	}
}

func (r *channelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
