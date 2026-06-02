package provider

import (
	"context"
	"fmt"

	"github.com/alexey-mylnikov/passwork-go/passwork"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ItemResource{}
var _ resource.ResourceWithImportState = &ItemResource{}

// ItemResource manages a Passwork password item.
type ItemResource struct {
	client *pwClient
}

// CustomFieldModel maps a single custom field.
type CustomFieldModel struct {
	Name  types.String `tfsdk:"name"`
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

// customFieldAttrTypes defines the attribute types for the custom_fields list element.
var customFieldAttrTypes = map[string]attr.Type{
	"name":  types.StringType,
	"type":  types.StringType,
	"value": types.StringType,
}

// ItemResourceModel maps the item resource schema.
type ItemResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	VaultID      types.String `tfsdk:"vault_id"`
	FolderID     types.String `tfsdk:"folder_id"`
	Login        types.String `tfsdk:"login"`
	Password     types.String `tfsdk:"password"`
	URL          types.String `tfsdk:"url"`
	Description  types.String `tfsdk:"description"`
	Tags         types.List   `tfsdk:"tags"`
	CustomFields types.List   `tfsdk:"custom_fields"`
}

// NewItemResource returns a new ItemResource factory.
func NewItemResource() resource.Resource {
	return &ItemResource{}
}

// Metadata returns the resource type name.
func (r *ItemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_item"
}

// Schema defines the item resource attributes.
func (r *ItemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Passwork password item.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Item identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the item.",
				Required:            true,
			},
			"vault_id": schema.StringAttribute{
				MarkdownDescription: "ID of the vault this item belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"folder_id": schema.StringAttribute{
				MarkdownDescription: "ID of the folder this item belongs to. Omit for vault root.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"login": schema.StringAttribute{
				MarkdownDescription: "Username / login.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password value. Stored in Passwork with client-side encryption when the master password is configured.",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Associated URL.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "List of tags.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"custom_fields": schema.ListNestedAttribute{
				MarkdownDescription: "List of custom fields.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Field name.",
							Required:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "Field type (e.g. `text`, `password`).",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Field value.",
							Required:            true,
							Sensitive:           true,
						},
					},
				},
			},
		},
	}
}

// Configure stores the provider client.
func (r *ItemResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*pwClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *pwClient, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// Create creates a new password item.
func (r *ItemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ItemResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item := modelToItem(ctx, plan)
	id, err := r.client.CreateItem(ctx, item)
	if err != nil {
		resp.Diagnostics.AddError("Error creating item", err.Error())
		return
	}

	// Read back from API so all computed fields (FolderID, Login, URL, Tags, etc.) are known.
	created, err := r.client.GetItem(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading item after create", err.Error())
		return
	}

	state, diags := itemToModel(ctx, created)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ID = types.StringValue(id)

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes item state from the API.
func (r *ItemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ItemResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item, err := r.client.GetItem(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading item", err.Error())
		return
	}

	updated, diags := itemToModel(ctx, item)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated.ID = state.ID

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}

// Update modifies an existing password item.
func (r *ItemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ItemResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item := modelToItem(ctx, plan)
	if err := r.client.UpdateItem(ctx, plan.ID.ValueString(), item); err != nil {
		resp.Diagnostics.AddError("Error updating item", err.Error())
		return
	}

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete moves the item to the bin.
func (r *ItemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ItemResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.DeleteItem(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting item", err.Error())
		return
	}
	r.client.saveSession()
}

// ImportState imports an item by ID.
func (r *ItemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	item, err := r.client.GetItem(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing item", err.Error())
		return
	}

	state, diags := itemToModel(ctx, item)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ID = types.StringValue(item.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// modelToItem converts an ItemResourceModel into a passwork.Item for API calls.
func modelToItem(ctx context.Context, m ItemResourceModel) passwork.Item {
	item := passwork.Item{
		VaultID:     m.VaultID.ValueString(),
		FolderID:    m.FolderID.ValueString(),
		Name:        m.Name.ValueString(),
		Login:       m.Login.ValueString(),
		Password:    m.Password.ValueString(),
		URL:         m.URL.ValueString(),
		Description: m.Description.ValueString(),
	}

	if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
		var tags []string
		_ = m.Tags.ElementsAs(ctx, &tags, false)
		item.Tags = tags
	}

	if !m.CustomFields.IsNull() && !m.CustomFields.IsUnknown() {
		var cfModels []CustomFieldModel
		_ = m.CustomFields.ElementsAs(ctx, &cfModels, false)
		for _, cf := range cfModels {
			item.Customs = append(item.Customs, passwork.CustomField{
				Name:  cf.Name.ValueString(),
				Type:  cf.Type.ValueString(),
				Value: cf.Value.ValueString(),
			})
		}
	}

	return item
}

// itemToModel converts a passwork.Item returned by the API into an ItemResourceModel.
func itemToModel(ctx context.Context, item *passwork.Item) (ItemResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	itemTags := item.Tags
	if itemTags == nil {
		itemTags = []string{}
	}
	tags, d := types.ListValueFrom(ctx, types.StringType, itemTags)
	diags.Append(d...)

	cfValues := make([]attr.Value, 0, len(item.Customs))
	for _, cf := range item.Customs {
		obj, d := types.ObjectValue(customFieldAttrTypes, map[string]attr.Value{
			"name":  types.StringValue(cf.Name),
			"type":  types.StringValue(cf.Type),
			"value": types.StringValue(cf.Value),
		})
		diags.Append(d...)
		cfValues = append(cfValues, obj)
	}

	cfList, d := types.ListValue(
		types.ObjectType{AttrTypes: customFieldAttrTypes},
		cfValues,
	)
	diags.Append(d...)

	m := ItemResourceModel{
		Name:         types.StringValue(item.Name),
		VaultID:      types.StringValue(item.VaultID),
		FolderID:     types.StringValue(item.FolderID),
		Login:        types.StringValue(item.Login),
		Password:     types.StringValue(item.Password),
		URL:          types.StringValue(item.URL),
		Description:  types.StringValue(item.Description),
		Tags:         tags,
		CustomFields: cfList,
	}
	return m, diags
}
