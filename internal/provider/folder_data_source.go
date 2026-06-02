package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &FolderDataSource{}

// FolderDataSource reads a folder by name within a vault.
type FolderDataSource struct {
	client *pwClient
}

// FolderDataSourceModel maps the folder data source schema.
type FolderDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	VaultID        types.String `tfsdk:"vault_id"`
	ParentFolderID types.String `tfsdk:"parent_folder_id"`
}

// NewFolderDataSource returns a new FolderDataSource factory.
func NewFolderDataSource() datasource.DataSource {
	return &FolderDataSource{}
}

// Metadata returns the data source type name.
func (d *FolderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Schema defines the data source attributes.
func (d *FolderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Passwork folder by name within a specific vault.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Folder identifier.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the folder to look up.",
				Required:            true,
			},
			"vault_id": schema.StringAttribute{
				MarkdownDescription: "ID of the vault to search within.",
				Required:            true,
			},
			"parent_folder_id": schema.StringAttribute{
				MarkdownDescription: "ID of the parent folder.",
				Computed:            true,
			},
		},
	}
}

// Configure stores the provider client.
func (d *FolderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read fetches the folder list for the vault and returns the first matching folder by name.
func (d *FolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg FolderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	folders, err := d.client.GetFolders(ctx, cfg.VaultID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing folders", err.Error())
		return
	}

	name := cfg.Name.ValueString()
	for _, f := range folders {
		if f.Name == name {
			cfg.ID = types.StringValue(f.ID)
			cfg.ParentFolderID = types.StringValue(f.ParentFolderID)
			resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
			return
		}
	}

	resp.Diagnostics.AddError("Folder not found", fmt.Sprintf("no folder with name %q in vault %q", name, cfg.VaultID.ValueString()))
}
