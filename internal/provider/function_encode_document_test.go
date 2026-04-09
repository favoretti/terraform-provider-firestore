package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func setupEncodeDocumentTest(t *testing.T, fields attr.Value, docType string, schemaVersion string) (
	context.Context, function.RunRequest, *function.RunResponse,
) {
	t.Helper()
	ctx := context.Background()

	dynamicFields := types.DynamicValue(fields)

	args := function.NewArgumentsData([]attr.Value{
		dynamicFields,
		types.StringValue(docType),
		types.StringValue(schemaVersion),
	})

	req := function.RunRequest{Arguments: args}
	resp := &function.RunResponse{
		Result: function.NewResultData(types.StringNull()),
	}

	return ctx, req, resp
}

// encode_document merges document_type and schema_version into the output.
func TestEncodeDocumentFunction_basic(t *testing.T) {
	fields, _ := types.ObjectValue(
		map[string]attr.Type{"name": types.StringType, "role": types.StringType},
		map[string]attr.Value{
			"name": types.StringValue("Alice"),
			"role": types.StringValue("admin"),
		},
	)

	f := &EncodeDocumentFunction{}
	ctx, req, resp := setupEncodeDocumentTest(t, fields, "custom-role", "1.1")

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}

	resultVal := resp.Result.Value()
	strVal, ok := resultVal.(types.String)
	if !ok {
		t.Fatalf("expected String result, got %T", resultVal)
	}
	resultStr := strVal.ValueString()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %s", err)
	}

	if parsed["document_type"] != "custom-role" {
		t.Fatalf("expected document_type=custom-role, got %v", parsed["document_type"])
	}
	if parsed["schema_version"] != "1.1" {
		t.Fatalf("expected schema_version=1.1, got %v", parsed["schema_version"])
	}
	if parsed["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", parsed["name"])
	}
	if parsed["role"] != "admin" {
		t.Fatalf("expected role=admin, got %v", parsed["role"])
	}
}

// FM 28: encode_document rejects empty document_type.
func TestEncodeDocumentFunction_emptyDocType(t *testing.T) {
	fields, _ := types.ObjectValue(
		map[string]attr.Type{"name": types.StringType},
		map[string]attr.Value{"name": types.StringValue("test")},
	)

	f := &EncodeDocumentFunction{}
	ctx, req, resp := setupEncodeDocumentTest(t, fields, "", "1.0")

	f.Run(ctx, req, resp)

	if resp.Error == nil {
		t.Fatal("expected error for empty document_type")
	}
	if resp.Error.Text != "document_type must not be empty" {
		t.Fatalf("unexpected error message: %s", resp.Error.Text)
	}
}

// FM 28: encode_document rejects empty schema_version.
func TestEncodeDocumentFunction_emptySchemaVersion(t *testing.T) {
	fields, _ := types.ObjectValue(
		map[string]attr.Type{"name": types.StringType},
		map[string]attr.Value{"name": types.StringValue("test")},
	)

	f := &EncodeDocumentFunction{}
	ctx, req, resp := setupEncodeDocumentTest(t, fields, "folder", "")

	f.Run(ctx, req, resp)

	if resp.Error == nil {
		t.Fatal("expected error for empty schema_version")
	}
	if resp.Error.Text != "schema_version must not be empty" {
		t.Fatalf("unexpected error message: %s", resp.Error.Text)
	}
}

// encode_document handles nested objects.
func TestEncodeDocumentFunction_nestedObject(t *testing.T) {
	innerObj, _ := types.ObjectValue(
		map[string]attr.Type{"key": types.StringType},
		map[string]attr.Value{"key": types.StringValue("val")},
	)
	fields, _ := types.ObjectValue(
		map[string]attr.Type{
			"name": types.StringType,
			"meta": innerObj.Type(nil),
		},
		map[string]attr.Value{
			"name": types.StringValue("test"),
			"meta": innerObj,
		},
	)

	f := &EncodeDocumentFunction{}
	ctx, req, resp := setupEncodeDocumentTest(t, fields, "vpc", "1.0")

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}

	resultVal := resp.Result.Value()
	strVal, ok := resultVal.(types.String)
	if !ok {
		t.Fatalf("expected String result, got %T", resultVal)
	}
	resultStr := strVal.ValueString()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %s", err)
	}

	meta, ok2 := parsed["meta"].(map[string]interface{})
	if !ok2 {
		t.Fatalf("expected meta to be a map, got %T", parsed["meta"])
	}
	if meta["key"] != "val" {
		t.Fatalf("expected meta.key=val, got %v", meta["key"])
	}
}

// encode_document handles null fields input by producing metadata-only output.
func TestEncodeDocumentFunction_nullFields(t *testing.T) {
	ctx := context.Background()
	f := &EncodeDocumentFunction{}

	nullDynamic := types.DynamicNull()

	args := function.NewArgumentsData([]attr.Value{
		nullDynamic,
		types.StringValue("folder"),
		types.StringValue("1.0"),
	})

	req := function.RunRequest{Arguments: args}
	resp := &function.RunResponse{
		Result: function.NewResultData(types.StringNull()),
	}

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}

	resultVal := resp.Result.Value()
	strVal, ok := resultVal.(types.String)
	if !ok {
		t.Fatalf("expected String result, got %T", resultVal)
	}
	resultStr := strVal.ValueString()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %s", err)
	}
	if parsed["document_type"] != "folder" {
		t.Fatalf("expected document_type=folder, got %v", parsed["document_type"])
	}
	if parsed["schema_version"] != "1.0" {
		t.Fatalf("expected schema_version=1.0, got %v", parsed["schema_version"])
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 keys (metadata only), got %d", len(parsed))
	}
}

// --- attrValueToGoValue tests ---

func TestAttrValueToGoValue_string(t *testing.T) {
	val, err := attrValueToGoValue(types.StringValue("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	s, ok := val.(string)
	if !ok || s != "hello" {
		t.Fatalf("expected 'hello', got %v", val)
	}
}

func TestAttrValueToGoValue_bool(t *testing.T) {
	val, err := attrValueToGoValue(types.BoolValue(true))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	b, ok := val.(bool)
	if !ok || !b {
		t.Fatalf("expected true, got %v", val)
	}
}

func TestAttrValueToGoValue_null(t *testing.T) {
	val, err := attrValueToGoValue(types.StringNull())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if val != nil {
		t.Fatalf("expected nil, got %v", val)
	}
}
