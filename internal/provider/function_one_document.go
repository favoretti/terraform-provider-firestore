package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = &OneDocumentFunction{}

type OneDocumentFunction struct{}

func NewOneDocumentFunction() function.Function {
	return &OneDocumentFunction{}
}

func (f *OneDocumentFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "one_document"
}

func (f *OneDocumentFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Extract the decoded fields from a single-document list",
		Description: "Takes the documents list from a firestore_documents data source that is expected to contain zero or one document. Returns the decoded fields object of the single document, or an empty object if the list is empty. Returns an error if the list contains more than one document.",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:        "documents",
				Description: "The documents list from a firestore_documents data source (expected to contain 0 or 1 document)",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (f *OneDocumentFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var docsVal types.Dynamic
	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &docsVal))
	if resp.Error != nil {
		return
	}

	underlyingVal := docsVal.UnderlyingValue()

	if underlyingVal == nil || underlyingVal.IsNull() || underlyingVal.IsUnknown() {
		emptyObj, diags := types.ObjectValue(map[string]attr.Type{}, map[string]attr.Value{})
		if diags.HasError() {
			resp.Error = function.NewFuncError("failed to create empty object")
			return
		}
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, types.DynamicValue(emptyObj)))
		return
	}

	var elements []attr.Value
	switch v := underlyingVal.(type) {
	case types.List:
		elements = v.Elements()
	case types.Tuple:
		elements = v.Elements()
	default:
		resp.Error = function.NewFuncError(
			fmt.Sprintf("expected a list of documents, got %T", underlyingVal))
		return
	}

	if len(elements) == 0 {
		emptyObj, diags := types.ObjectValue(map[string]attr.Type{}, map[string]attr.Value{})
		if diags.HasError() {
			resp.Error = function.NewFuncError("failed to create empty object")
			return
		}
		resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, types.DynamicValue(emptyObj)))
		return
	}

	if len(elements) > 1 {
		resp.Error = function.NewFuncError(
			fmt.Sprintf("expected 0 or 1 document, got %d; add a where filter to narrow the results", len(elements)))
		return
	}

	objVal, ok := elements[0].(types.Object)
	if !ok {
		resp.Error = function.NewFuncError(
			fmt.Sprintf("expected a document object, got %T", elements[0]))
		return
	}

	attrs := objVal.Attributes()
	fieldsAttr, exists := attrs["fields"]
	if !exists {
		resp.Error = function.NewFuncError("document object missing fields attribute")
		return
	}

	fieldsStr, ok := fieldsAttr.(types.String)
	if !ok || fieldsStr.IsNull() || fieldsStr.IsUnknown() {
		resp.Error = function.NewFuncError("document fields must be a non-null string")
		return
	}

	var raw interface{}
	if err := json.Unmarshal([]byte(fieldsStr.ValueString()), &raw); err != nil {
		resp.Error = function.NewFuncError(
			fmt.Sprintf("failed to parse fields JSON: %s", err.Error()))
		return
	}

	tfVal, err := goValueToTerraformValue(raw)
	if err != nil {
		resp.Error = function.NewFuncError(
			fmt.Sprintf("failed to convert fields: %s", err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, tfVal))
}
