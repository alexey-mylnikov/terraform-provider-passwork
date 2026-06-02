package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &VaultTypeDataSource{}

// VaultTypeDataSource looks up a vault type by name or code.
type VaultTypeDataSource struct {
	client *pwClient
}

// VaultTypeDataSourceModel maps the vault type data source schema.
type VaultTypeDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Code types.String `tfsdk:"code"`
}

// NewVaultTypeDataSource returns a new VaultTypeDataSource factory.
func NewVaultTypeDataSource() datasource.DataSource {
	return &VaultTypeDataSource{}
}

// Metadata returns the data source type name.
func (d *VaultTypeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault_type"
}

// Schema defines the data source attributes.
func (d *VaultTypeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Passwork vault type by name or code. Use the returned `id` as `type_id` in `passwork_vault` when the server has the `vaultType` feature enabled.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Vault type identifier.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Vault type display name. Specify either `name` or `code`.",
				Optional:            true,
				Computed:            true,
			},
			"code": schema.StringAttribute{
				MarkdownDescription: "Vault type code. Specify either `name` or `code`.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

// Configure stores the provider client.
func (d *VaultTypeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*pwClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *pwClient, got %T", req.ProviderData))
		return
	}
	d.client = c
}

// Read fetches the vault type list and returns the first entry matching name or code.
func (d *VaultTypeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg VaultTypeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameSet := !cfg.Name.IsNull() && !cfg.Name.IsUnknown() && cfg.Name.ValueString() != ""
	codeSet := !cfg.Code.IsNull() && !cfg.Code.IsUnknown() && cfg.Code.ValueString() != ""

	if !nameSet && !codeSet {
		resp.Diagnostics.AddError("Missing search attribute", "Specify at least one of `name` or `code`.")
		return
	}

	types_, err := d.client.GetVaultTypes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing vault types", err.Error())
		return
	}

	for _, vt := range types_ {
		if (nameSet && vt.Name == cfg.Name.ValueString()) ||
			(codeSet && vt.Code == cfg.Code.ValueString()) {
			cfg.ID = types.StringValue(vt.ID)
			cfg.Name = types.StringValue(vt.Name)
			cfg.Code = types.StringValue(vt.Code)
			resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
			return
		}
	}

	filter := cfg.Name.ValueString()
	if filter == "" {
		filter = cfg.Code.ValueString()
	}
	resp.Diagnostics.AddError("Vault type not found", fmt.Sprintf("no vault type matching %q", filter))
}
