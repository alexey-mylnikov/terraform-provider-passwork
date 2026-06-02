package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &VaultDataSource{}

// VaultDataSource reads a vault by name.
type VaultDataSource struct {
	client *pwClient
}

// VaultDataSourceModel maps the vault data source schema.
type VaultDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	TypeID types.String `tfsdk:"type_id"`
}

// NewVaultDataSource returns a new VaultDataSource factory.
func NewVaultDataSource() datasource.DataSource {
	return &VaultDataSource{}
}

// Metadata returns the data source type name.
func (d *VaultDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

// Schema defines the data source attributes.
func (d *VaultDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Passwork vault by name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Vault identifier.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the vault to look up.",
				Required:            true,
			},
			"type_id": schema.StringAttribute{
				MarkdownDescription: "Vault type template ID.",
				Computed:            true,
			},
		},
	}
}

// Configure stores the provider client.
func (d *VaultDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read fetches the vault list and returns the first vault whose name matches.
func (d *VaultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg VaultDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vaults, err := d.client.GetVaults(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing vaults", err.Error())
		return
	}

	name := cfg.Name.ValueString()
	for _, v := range vaults {
		if v.Name == name {
			cfg.ID = types.StringValue(v.ID)
			cfg.TypeID = types.StringValue(v.TypeID)
			resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
			return
		}
	}

	resp.Diagnostics.AddError("Vault not found", fmt.Sprintf("no vault with name %q", name))
}
