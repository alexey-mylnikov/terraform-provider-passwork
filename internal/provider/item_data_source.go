package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ItemDataSource{}

// ItemDataSource reads a password item by ID.
type ItemDataSource struct {
	client *pwClient
}

// NewItemDataSource returns a new ItemDataSource factory.
func NewItemDataSource() datasource.DataSource {
	return &ItemDataSource{}
}

// Metadata returns the data source type name.
func (d *ItemDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_item"
}

// Schema defines the data source attributes.
func (d *ItemDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Passwork password item by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Item identifier.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name.",
				Computed:            true,
			},
			"vault_id": schema.StringAttribute{
				MarkdownDescription: "Vault the item belongs to.",
				Computed:            true,
			},
			"folder_id": schema.StringAttribute{
				MarkdownDescription: "Folder the item belongs to.",
				Computed:            true,
			},
			"login": schema.StringAttribute{
				MarkdownDescription: "Username / login.",
				Computed:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password value.",
				Computed:            true,
				Sensitive:           true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Associated URL.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Computed:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Tags.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"custom_fields": schema.ListNestedAttribute{
				MarkdownDescription: "Custom fields.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Field name.",
							Computed:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "Field type.",
							Computed:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Field value.",
							Computed:            true,
							Sensitive:           true,
						},
					},
				},
			},
		},
	}
}

// Configure stores the provider client.
func (d *ItemDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read fetches the item from the API.
func (d *ItemDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg ItemResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item, err := d.client.GetItem(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading item", err.Error())
		return
	}

	state, diags := itemToModel(ctx, item)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ID = cfg.ID

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
