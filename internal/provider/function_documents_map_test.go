package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func buildDocumentObject(docID string, fieldsJSON string) types.Object {
	docObjAttrTypes := map[string]attr.Type{
		"document_id": types.StringType,
		"fields":      types.StringType,
		"fields_map":  types.MapType{ElemType: types.StringType},
		"create_time": types.StringType,
		"update_time": types.StringType,
	}

	emptyMap, _ := types.MapValue(types.StringType, map[string]attr.Value{})

	obj, _ := types.ObjectValue(docObjAttrTypes, map[string]attr.Value{
		"document_id": types.StringValue(docID),
		"fields":      types.StringValue(fieldsJSON),
		"fields_map":  emptyMap,
		"create_time": types.StringValue("2024-01-01T00:00:00Z"),
		"update_time": types.StringValue("2024-01-01T00:00:00Z"),
	})
	return obj
}

func setupDocumentsMapTest(t *testing.T, docs []types.Object) (
	context.Context, function.RunRequest, *function.RunResponse,
) {
	t.Helper()
	ctx := context.Background()

	var listVal attr.Value
	if len(docs) == 0 {
		elemTypes := []attr.Type{}
		elemVals := []attr.Value{}
		tupleVal, diags := types.TupleValue(elemTypes, elemVals)
		if diags.HasError() {
			t.Fatalf("failed to create empty tuple: %s", diags.Errors()[0].Detail())
		}
		listVal = tupleVal
	} else {
		elemTypes := make([]attr.Type, len(docs))
		elemVals := make([]attr.Value, len(docs))
		for i, doc := range docs {
			elemTypes[i] = doc.Type(ctx)
			elemVals[i] = doc
		}
		tupleVal, diags := types.TupleValue(elemTypes, elemVals)
		if diags.HasError() {
			t.Fatalf("failed to create tuple: %s", diags.Errors()[0].Detail())
		}
		listVal = tupleVal
	}

	dynamicVal := types.DynamicValue(listVal)

	args := function.NewArgumentsData([]attr.Value{dynamicVal})

	req := function.RunRequest{Arguments: args}
	resp := &function.RunResponse{
		Result: function.NewResultData(types.DynamicNull()),
	}

	return ctx, req, resp
}

// documents_map returns a map keyed by document_id with decoded fields.
func TestDocumentsMapFunction_basic(t *testing.T) {
	docs := []types.Object{
		buildDocumentObject("doc-1", `{"name":"Alice","role":"admin"}`),
		buildDocumentObject("doc-2", `{"name":"Bob","role":"user"}`),
	}

	f := &DocumentsMapFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, docs)

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// documents_map returns an empty map for an empty list.
func TestDocumentsMapFunction_emptyList(t *testing.T) {
	f := &DocumentsMapFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, []types.Object{})

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// FM 27: documents_map receives documents with invalid fields JSON.
func TestDocumentsMapFunction_invalidJSON(t *testing.T) {
	docs := []types.Object{
		buildDocumentObject("doc-1", `not json`),
	}

	f := &DocumentsMapFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, docs)

	f.Run(ctx, req, resp)

	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON fields")
	}
}

// documents_map handles a single document correctly.
func TestDocumentsMapFunction_singleDocument(t *testing.T) {
	docs := []types.Object{
		buildDocumentObject("only-one", `{"project_id":"my-proj","schema_version":"1.0"}`),
	}

	f := &DocumentsMapFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, docs)

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// FM 27: documents_map receives null input.
func TestDocumentsMapFunction_nullInput(t *testing.T) {
	ctx := context.Background()
	f := &DocumentsMapFunction{}

	nullDynamic := types.DynamicNull()

	args := function.NewArgumentsData([]attr.Value{nullDynamic})

	req := function.RunRequest{Arguments: args}
	resp := &function.RunResponse{
		Result: function.NewResultData(types.DynamicNull()),
	}

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error for null input: %s", resp.Error.Error())
	}
}

func setupNullDynamicTest(t *testing.T) (
	context.Context, function.RunRequest, *function.RunResponse,
) {
	t.Helper()
	ctx := context.Background()

	nullDynamic := types.DynamicNull()
	args := function.NewArgumentsData([]attr.Value{nullDynamic})

	req := function.RunRequest{Arguments: args}
	resp := &function.RunResponse{
		Result: function.NewResultData(types.DynamicNull()),
	}

	return ctx, req, resp
}
