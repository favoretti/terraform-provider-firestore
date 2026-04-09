package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = &DecodeFunction{}

type DecodeFunction struct{}

func NewDecodeFunction() function.Function {
	return &DecodeFunction{}
}

func (f *DecodeFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "decode"
}

func (f *DecodeFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Decode a Firestore document fields JSON string into an HCL object",
		Description: "Takes the fields JSON string from a firestore_document or firestore_documents data source and returns the decoded object with native HCL types.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:        "fields_json",
				Description: "The JSON string from a document's fields attribute",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (f *DecodeFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var fieldsJSON string
	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &fieldsJSON))
	if resp.Error != nil {
		return
	}

	if fieldsJSON == "" {
		resp.Error = function.NewFuncError("fields_json must not be empty")
		return
	}

	var raw interface{}
	if err := json.Unmarshal([]byte(fieldsJSON), &raw); err != nil {
		resp.Error = function.NewFuncError(fmt.Sprintf("failed to parse fields JSON: %s", err.Error()))
		return
	}

	tfVal, err := goValueToTerraformValue(raw)
	if err != nil {
		resp.Error = function.NewFuncError(fmt.Sprintf("failed to convert fields to Terraform value: %s", err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, types.DynamicValue(tfVal)))
}
