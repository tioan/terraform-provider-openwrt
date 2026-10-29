// Copyright https://github.com/Foxboron/terraform-provider-openwrt/graphs/contributors 2025, 2026
// SPDX-License-Identifier: MPL-2.0

package fs

import (
	"bytes"
	"context"
	"fmt"
	"path"

	rpc "github.com/foxboron/terraform-provider-openwrt/internal/api/luci"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var etcConfig = "/etc/config"

type configFileModel struct {
	Name    types.String `tfsdk:"name"`
	Content types.String `tfsdk:"content"`
	Commit  types.Bool   `tfsdk:"commit"`
}

// configFileResource represent Incus project resource.
type configFileResource struct {
	provider rpc.RPC
}

// NewProjectResource return new project resource.
func NewConfigFileResource() resource.Resource {
	return &configFileResource{}
}

// Metadata for project resource.
func (c configFileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_configfile", req.ProviderTypeName)
}

func (c configFileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Write configuration files to `/etc/config` on the OpenWRT router.",
		Description:         "Write configuration files to /etc/config on the OpenWRT router.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the configuration file.",
				Description:         "Name of the configuration file.",
				Required:            true,
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "The content of the configuration file.",
				Description:         "The content of the configuration file.",
				Required:            true,
			},
			"commit": schema.BoolAttribute{
				MarkdownDescription: "If we should tell `uci` to run `commit` on the configuration file. (Default: true)",
				Description:         "If we should tell uci to run commit on the configuration file. (Default: true)",
				Optional:            true,
				Default:             booldefault.StaticBool(true),
				Computed:            true,
			},
		},
	}
}

func (c *configFileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := req.ProviderData
	if data == nil {
		return
	}
	provider, ok := data.(rpc.RPC)
	if !ok {
		resp.Diagnostics.AddError("Failed to get api client", "")
		return
	}
	c.provider = provider
}

func (c configFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan configFileModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := path.Join(etcConfig, plan.Name.ValueString())

	if err := c.provider.Writefile(ctx, path, []byte(plan.Content.ValueString())); err != nil {
		resp.Diagnostics.AddError("Failed to write file", err.Error())
		return
	}

	if plan.Commit.ValueBool() {
		if err := c.provider.CommitOrRevert(ctx, plan.Name.ValueString()); err != nil {
			resp.Diagnostics.AddError("failed to commit or revert", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (c configFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state configFileModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := path.Join(etcConfig, state.Name.ValueString())
	b, err := c.provider.ReadFile(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Failed to read config %q", state.Name.ValueString()), err.Error())
		return
	}

	// Logic taken from the local_file provider
	if !bytes.Equal(b, []byte(state.Content.ValueString())) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (c configFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan configFileModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := path.Join(etcConfig, plan.Name.ValueString())
	if err := c.provider.Writefile(ctx, path, []byte(plan.Content.ValueString())); err != nil {
		resp.Diagnostics.AddError("Failed to write file", err.Error())
	}

	if plan.Commit.ValueBool() {
		if err := c.provider.CommitOrRevert(ctx, plan.Name.ValueString()); err != nil {
			resp.Diagnostics.AddError("failed to commit or revert", err.Error())
			return
		}
	}

	plan.Commit = types.BoolValue(true)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (c configFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state configFileModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Commit before removing the file: uci commit and revert both fail on a
	// config that no longer exists. This also flushes changes staged for the
	// config, so they cannot bring it back later.
	if state.Commit.ValueBool() {
		if err := c.provider.CommitOrRevert(ctx, state.Name.ValueString()); err != nil {
			resp.Diagnostics.AddError("failed to commit or revert", err.Error())
			return
		}
	}

	path := path.Join(etcConfig, state.Name.ValueString())
	err := c.provider.RemoveFile(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Failed to delete config file %q", state.Name.ValueString()), err.Error())
		return
	}
}

func (c *configFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var state configFileModel

	path := path.Join(etcConfig, req.ID)
	b, err := c.provider.ReadFile(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Failed to read config %q", state.Name.ValueString()), err.Error())
		return
	}

	state.Name = types.StringValue(req.ID)
	state.Content = types.StringValue(string(b))
	state.Commit = types.BoolValue(true)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
