package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = &EncodeDocumentFunction{}

type EncodeDocumentFunction struct{}

func NewEncodeDocumentFunction() function.Function {
	return &EncodeDocumentFunction{}
}

func (f *EncodeDocumentFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "encode_document"
}

func (f *EncodeDocumentFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Encode fields with document_type and schema_version metadata into a JSON string",
		Description: "Takes an HCL object of field values, a document_type string, and a schema_version string. Returns a JSON string suitable for the firestore_document resource's fields attribute, with document_type and schema_version merged into the object.",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:        "fields",
				Description: "The document field values as an HCL object or map",
			},
			function.StringParameter{
				Name:        "document_type",
				Description: "The document_type value to include in the encoded document",
			},
			function.StringParameter{
				Name:        "schema_version",
				Description: "The schema_version value to include in the encoded document",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *EncodeDocumentFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var fieldsVal types.Dynamic
	var docType string
	var schemaVersion string

	resp.Error = function.ConcatFuncErrors(
		req.Arguments.Get(ctx, &fieldsVal, &docType, &schemaVersion),
	)
	if resp.Error != nil {
		return
	}

	if docType == "" {
		resp.Error = function.NewFuncError("document_type must not be empty")
		return
	}
	if schemaVersion == "" {
		resp.Error = function.NewFuncError("schema_version must not be empty")
		return
	}

	goVal, err := attrValueToGoValue(fieldsVal.UnderlyingValue())
	if err != nil {
		resp.Error = function.NewFuncError(
			fmt.Sprintf("failed to convert fields to Go value: %s", err.Error()))
		return
	}

	merged, ok := goVal.(map[string]interface{})
	if !ok {
		if goVal == nil {
			merged = make(map[string]interface{})
		} else {
			resp.Error = function.NewFuncError(
				fmt.Sprintf("fields must be an object/map, got %T", goVal))
			return
		}
	}

	merged["document_type"] = docType
	merged["schema_version"] = schemaVersion

	jsonBytes, err := json.Marshal(merged)
	if err != nil {
		resp.Error = function.NewFuncError(
			fmt.Sprintf("failed to encode fields as JSON: %s", err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, string(jsonBytes)))
}

func attrValueToGoValue(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case nil:
		return nil, nil
	case types.String:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		return val.ValueString(), nil
	case types.Bool:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		return val.ValueBool(), nil
	case types.Number:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		bf := val.ValueBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return i, nil
		}
		f, _ := bf.Float64()
		return f, nil
	case types.Int64:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		return val.ValueInt64(), nil
	case types.Float64:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		return val.ValueFloat64(), nil
	case types.Object:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		attrs := val.Attributes()
		result := make(map[string]interface{}, len(attrs))
		for k, v := range attrs {
			child, err := attrValueToGoValue(v)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", k, err)
			}
			result[k] = child
		}
		return result, nil
	case types.Map:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		elems := val.Elements()
		result := make(map[string]interface{}, len(elems))
		for k, v := range elems {
			child, err := attrValueToGoValue(v)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", k, err)
			}
			result[k] = child
		}
		return result, nil
	case types.List:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		elems := val.Elements()
		result := make([]interface{}, len(elems))
		for i, e := range elems {
			child, err := attrValueToGoValue(e)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = child
		}
		return result, nil
	case types.Tuple:
		if val.IsNull() || val.IsUnknown() {
			return nil, nil
		}
		elems := val.Elements()
		result := make([]interface{}, len(elems))
		for i, e := range elems {
			child, err := attrValueToGoValue(e)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = child
		}
		return result, nil
	case types.Dynamic:
		return attrValueToGoValue(val.UnderlyingValue())
	default:
		return nil, fmt.Errorf("unsupported attribute type %T", v)
	}
}
