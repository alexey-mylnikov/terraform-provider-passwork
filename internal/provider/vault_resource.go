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

var _ resource.Resource = &VaultResource{}
var _ resource.ResourceWithImportState = &VaultResource{}

// VaultResource manages a Passwork vault.
type VaultResource struct {
	client *pwClient
}

// VaultResourceModel maps the vault resource schema.
type VaultResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	TypeID types.String `tfsdk:"type_id"`
}

// NewVaultResource returns a new VaultResource factory.
func NewVaultResource() resource.Resource {
	return &VaultResource{}
}

// Metadata returns the resource type name.
func (r *VaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

// Schema defines the vault resource attributes.
func (r *VaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Passwork vault.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Vault identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the vault.",
				Required:            true,
			},
			"type_id": schema.StringAttribute{
				MarkdownDescription: "Vault type template ID. Required when the server has the `vaultType` feature enabled.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// Configure stores the provider client.
func (r *VaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create creates a new vault.
func (r *VaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	typeID := ""
	if !plan.TypeID.IsNull() && !plan.TypeID.IsUnknown() {
		typeID = plan.TypeID.ValueString()
	}

	id, err := r.client.CreateVault(ctx, plan.Name.ValueString(), typeID)
	if err != nil {
		resp.Diagnostics.AddError("Error creating vault", err.Error())
		return
	}

	plan.ID = types.StringValue(id)
	if typeID == "" {
		plan.TypeID = types.StringValue("")
	}

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes vault state from the API.
func (r *VaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vault, err := r.client.GetVault(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading vault", err.Error())
		return
	}

	state.Name = types.StringValue(vault.Name)
	state.TypeID = types.StringValue(vault.TypeID)

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update renames an existing vault.
func (r *VaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateVault(ctx, plan.ID.ValueString(), plan.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error updating vault", err.Error())
		return
	}

	r.client.saveSession()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete moves the vault to the bin.
func (r *VaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state VaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteVault(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting vault", err.Error())
		return
	}
	r.client.saveSession()
}

// ImportState imports a vault by ID.
func (r *VaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	state := VaultResourceModel{ID: types.StringValue(req.ID)}

	vault, err := r.client.GetVault(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing vault", err.Error())
		return
	}
	state.Name = types.StringValue(vault.Name)
	state.TypeID = types.StringValue(vault.TypeID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
