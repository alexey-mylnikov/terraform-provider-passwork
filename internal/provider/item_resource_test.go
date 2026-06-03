package provider

import (
	"context"
	"testing"

	"github.com/alexey-mylnikov/passwork-go/passwork"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestItemToModel_BasicFields(t *testing.T) {
	ctx := context.Background()
	item := &passwork.Item{
		ID:          "item-1",
		Name:        "My Item",
		VaultID:     "vault-1",
		FolderID:    "folder-1",
		Login:       "user@example.com",
		Password:    "s3cr3t",
		URL:         "https://example.com",
		Description: "desc",
		Tags:        []string{"tag1", "tag2"},
	}

	m, diags := itemToModel(ctx, item)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if m.Name.ValueString() != "My Item" {
		t.Errorf("Name: want %q, got %q", "My Item", m.Name.ValueString())
	}
	if m.VaultID.ValueString() != "vault-1" {
		t.Errorf("VaultID: want %q, got %q", "vault-1", m.VaultID.ValueString())
	}
	if m.Password.ValueString() != "s3cr3t" {
		t.Errorf("Password: want %q, got %q", "s3cr3t", m.Password.ValueString())
	}
	var tags []string
	m.Tags.ElementsAs(ctx, &tags, false)
	if len(tags) != 2 || tags[0] != "tag1" || tags[1] != "tag2" {
		t.Errorf("Tags: want [tag1 tag2], got %v", tags)
	}
}

func TestItemToModel_CustomFields(t *testing.T) {
	ctx := context.Background()
	item := &passwork.Item{
		Name:    "With custom",
		VaultID: "vault-1",
		Tags:    nil,
		Customs: []passwork.CustomField{
			{Name: "field1", Type: "text", Value: "val1"},
			{Name: "field2", Type: "password", Value: "secret"},
		},
	}

	m, diags := itemToModel(ctx, item)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var cfs []CustomFieldModel
	m.CustomFields.ElementsAs(ctx, &cfs, false)
	if len(cfs) != 2 {
		t.Fatalf("want 2 custom fields, got %d", len(cfs))
	}
	if cfs[0].Name.ValueString() != "field1" {
		t.Errorf("CustomField[0].Name: want %q, got %q", "field1", cfs[0].Name.ValueString())
	}
	if cfs[1].Value.ValueString() != "secret" {
		t.Errorf("CustomField[1].Value: want %q, got %q", "secret", cfs[1].Value.ValueString())
	}
}

func TestModelToItem_RoundTrip(t *testing.T) {
	ctx := context.Background()

	cfObj, _ := types.ObjectValue(customFieldAttrTypes, map[string]attr.Value{
		"name":  types.StringValue("cf1"),
		"type":  types.StringValue("text"),
		"value": types.StringValue("cfval"),
	})
	cfList, _ := types.ListValue(types.ObjectType{AttrTypes: customFieldAttrTypes}, []attr.Value{cfObj})
	tagList, _ := types.ListValueFrom(ctx, types.StringType, []string{"t1"})

	m := ItemResourceModel{
		ID:           types.StringValue("id-1"),
		Name:         types.StringValue("test"),
		VaultID:      types.StringValue("v1"),
		FolderID:     types.StringValue("f1"),
		Login:        types.StringValue("login"),
		Password:     types.StringValue("pass"),
		URL:          types.StringValue("http://x.com"),
		Description:  types.StringValue("d"),
		Tags:         tagList,
		CustomFields: cfList,
	}

	item := modelToItem(ctx, m)

	if item.Name != "test" {
		t.Errorf("Name: want %q, got %q", "test", item.Name)
	}
	if item.Password != "pass" {
		t.Errorf("Password: want %q, got %q", "pass", item.Password)
	}
	if len(item.Tags) != 1 || item.Tags[0] != "t1" {
		t.Errorf("Tags: want [t1], got %v", item.Tags)
	}
	if len(item.Customs) != 1 || item.Customs[0].Name != "cf1" {
		t.Errorf("Customs: want [{cf1 text cfval}], got %v", item.Customs)
	}
}

func TestItemToModel_EmptyTags(t *testing.T) {
	ctx := context.Background()
	item := &passwork.Item{Name: "x", VaultID: "v", Tags: nil}

	m, diags := itemToModel(ctx, item)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if m.Tags.IsNull() || m.Tags.IsUnknown() {
		t.Error("Tags should be an empty list, not null/unknown")
	}
	if len(m.Tags.Elements()) != 0 {
		t.Errorf("Tags: want empty list, got %v", m.Tags.Elements())
	}
}
