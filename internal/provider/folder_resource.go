package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &FolderResource{}
var _ resource.ResourceWithImportState = &FolderResource{}

// FolderResource manages a Passwork folder.
type FolderResource struct {
	client *pwClient
}

// FolderResourceModel maps the folder resource schema.
type FolderResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	VaultID        types.String `tfsdk:"vault_id"`
	ParentFolderID types.String `tfsdk:"parent_folder_id"`
}

// NewFolderResource returns a new FolderResource factory.
func NewFolderResource() resource.Resource {
	return &FolderResource{}
}

// Metadata returns the resource type name.
func (r *FolderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Schema defines the folder resource attributes.
func (r *FolderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Passwork folder inside a vault.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Folder identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the folder.",
				Required:            true,
			},
			"vault_id": schema.StringAttribute{
				MarkdownDescription: "ID of the vault this folder belongs to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"parent_folder_id": schema.StringAttribute{
				MarkdownDescription: "ID of the parent folder. Omit for a root-level folder.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// Configure stores the provider client.
func (r *FolderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create creates a new folder.
func (r *FolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parentID := ""
	if !plan.ParentFolderID.IsNull() && !plan.ParentFolderID.IsUnknown() {
		parentID = plan.ParentFolderID.ValueString()
	}

	id, err := r.client.CreateFolder(ctx, plan.Name.ValueString(), plan.VaultID.ValueString(), parentID)
	if err != nil {
		resp.Diagnostics.AddError("Error creating folder", err.Error())
		return
	}

	folder, err := r.client.GetFolder(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading folder after create", err.Error())
		return
	}

	state := FolderResourceModel{
		ID:             types.StringValue(folder.ID),
		Name:           types.StringValue(folder.Name),
		VaultID:        types.StringValue(folder.VaultID),
		ParentFolderID: types.StringValue(folder.ParentFolderID),
	}

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes folder state from the API.
func (r *FolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FolderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	folder, err := r.client.GetFolder(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading folder", err.Error())
		return
	}

	state.Name = types.StringValue(folder.Name)
	state.VaultID = types.StringValue(folder.VaultID)
	state.ParentFolderID = types.StringValue(folder.ParentFolderID)

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update renames an existing folder.
func (r *FolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateFolder(ctx, plan.ID.ValueString(), plan.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error updating folder", err.Error())
		return
	}

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete moves the folder to the bin.
func (r *FolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FolderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.DeleteFolder(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting folder", err.Error())
		return
	}
	r.client.saveSession()
}

// ImportState imports a folder by ID.
func (r *FolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	folder, err := r.client.GetFolder(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing folder", err.Error())
		return
	}

	state := FolderResourceModel{
		ID:             types.StringValue(folder.ID),
		Name:           types.StringValue(folder.Name),
		VaultID:        types.StringValue(folder.VaultID),
		ParentFolderID: types.StringValue(folder.ParentFolderID),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
