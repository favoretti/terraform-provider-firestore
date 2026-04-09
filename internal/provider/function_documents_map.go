package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = &DocumentsMapFunction{}

type DocumentsMapFunction struct{}

func NewDocumentsMapFunction() function.Function {
	return &DocumentsMapFunction{}
}

func (f *DocumentsMapFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "documents_map"
}

func (f *DocumentsMapFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Convert a documents list into a map keyed by document_id",
		Description: "Takes the documents list from a firestore_documents data source and returns a map where each key is the document_id and each value is the decoded fields object. Replaces the common pattern: { for doc in docs : doc.document_id => jsondecode(doc.fields) }",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:        "documents",
				Description: "The documents list from a firestore_documents data source",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (f *DocumentsMapFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var docsVal types.Dynamic
	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &docsVal))
	if resp.Error != nil {
		return
	}

	underlyingVal := docsVal.UnderlyingValue()

	if underlyingVal == nil || underlyingVal.IsNull() || underlyingVal.IsUnknown() {
		emptyMap, diags := types.MapValue(types.DynamicType, map[string]attr.Value{})
		if diags.HasError() {
			resp.Error = function.NewFuncError("failed to create empty map")
			return
		}
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, types.DynamicValue(emptyMap)))
		return
	}

	listVal, ok := underlyingVal.(types.List)
	if !ok {
		tupleVal, tupleOk := underlyingVal.(types.Tuple)
		if !tupleOk {
			resp.Error = function.NewFuncError(
				fmt.Sprintf("expected a list of documents, got %T", underlyingVal))
			return
		}
		result, funcErr := processDocumentsTuple(tupleVal)
		if funcErr != nil {
			resp.Error = funcErr
			return
		}
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
		return
	}

	result, funcErr := processDocumentsList(listVal)
	if funcErr != nil {
		resp.Error = funcErr
		return
	}
	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, result))
}

func processDocumentsList(listVal types.List) (types.Dynamic, *function.FuncError) {
	elements := listVal.Elements()
	return processDocumentElements(elements)
}

func processDocumentsTuple(tupleVal types.Tuple) (types.Dynamic, *function.FuncError) {
	elements := tupleVal.Elements()
	return processDocumentElements(elements)
}

func processDocumentElements(elements []attr.Value) (types.Dynamic, *function.FuncError) {
	if len(elements) == 0 {
		emptyMap, diags := types.MapValue(types.DynamicType, map[string]attr.Value{})
		if diags.HasError() {
			return types.DynamicNull(), function.NewFuncError("failed to create empty map")
		}
		return types.DynamicValue(emptyMap), nil
	}

	resultMap := make(map[string]attr.Value, len(elements))

	for i, elem := range elements {
		objVal, ok := elem.(types.Object)
		if !ok {
			return types.DynamicNull(), function.NewFuncError(
				fmt.Sprintf("element %d: expected an object with document_id and fields attributes, got %T", i, elem))
		}

		attrs := objVal.Attributes()

		docIDAttr, exists := attrs["document_id"]
		if !exists {
			return types.DynamicNull(), function.NewFuncError(
				fmt.Sprintf("element %d: missing document_id attribute", i))
		}
		docIDStr, ok := docIDAttr.(types.String)
		if !ok || docIDStr.IsNull() || docIDStr.IsUnknown() {
			return types.DynamicNull(), function.NewFuncError(
				fmt.Sprintf("element %d: document_id must be a non-null string", i))
		}

		fieldsAttr, exists := attrs["fields"]
		if !exists {
			return types.DynamicNull(), function.NewFuncError(
				fmt.Sprintf("element %d: missing fields attribute", i))
		}
		fieldsStr, ok := fieldsAttr.(types.String)
		if !ok || fieldsStr.IsNull() || fieldsStr.IsUnknown() {
			return types.DynamicNull(), function.NewFuncError(
				fmt.Sprintf("element %d: fields must be a non-null string", i))
		}

		var raw interface{}
		if err := json.Unmarshal([]byte(fieldsStr.ValueString()), &raw); err != nil {
			return types.DynamicNull(), function.NewFuncError(
				fmt.Sprintf("element %d (document_id=%q): failed to parse fields JSON: %s",
					i, docIDStr.ValueString(), err.Error()))
		}

		tfVal, err := goValueToTerraformValue(raw)
		if err != nil {
			return types.DynamicNull(), function.NewFuncError(
				fmt.Sprintf("element %d (document_id=%q): failed to convert fields: %s",
					i, docIDStr.ValueString(), err.Error()))
		}

		resultMap[docIDStr.ValueString()] = tfVal
	}

	resultMapVal, diags := types.MapValue(types.DynamicType, resultMap)
	if diags.HasError() {
		return types.DynamicNull(), function.NewFuncError(
			fmt.Sprintf("failed to create result map: %s", diags.Errors()[0].Detail()))
	}

	return types.DynamicValue(resultMapVal), nil
}
