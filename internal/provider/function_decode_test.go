package provider

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FM 30: decode receives a non-JSON string.
func TestDecodeFunction_invalidJSON(t *testing.T) {
	f := &DecodeFunction{}
	ctx, req, resp := setupFunctionTest(t, f, "not valid json")

	f.Run(ctx, req, resp)

	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// FM 30: decode receives an empty string.
func TestDecodeFunction_emptyString(t *testing.T) {
	f := &DecodeFunction{}
	ctx, req, resp := setupFunctionTest(t, f, "")

	f.Run(ctx, req, resp)

	if resp.Error == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

// decode returns a flat object with string fields.
func TestDecodeFunction_flatObject(t *testing.T) {
	input := `{"name":"test","document_type":"folder"}`
	f := &DecodeFunction{}
	ctx, req, resp := setupFunctionTest(t, f, input)

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// decode handles nested objects and arrays.
func TestDecodeFunction_nestedStructure(t *testing.T) {
	input := `{"name":"test","tags":["a","b"],"meta":{"key":"val"}}`
	f := &DecodeFunction{}
	ctx, req, resp := setupFunctionTest(t, f, input)

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// decode handles numeric values.
func TestDecodeFunction_numericValues(t *testing.T) {
	input := `{"count":42,"price":19.99,"active":true,"nothing":null}`
	f := &DecodeFunction{}
	ctx, req, resp := setupFunctionTest(t, f, input)

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// --- goValueToTerraformValue tests ---

func TestGoValueToTerraformValue_string(t *testing.T) {
	val, err := goValueToTerraformValue("hello")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	strVal, ok := underlying.(types.String)
	if !ok {
		t.Fatalf("expected String, got %T", underlying)
	}
	if strVal.ValueString() != "hello" {
		t.Fatalf("expected 'hello', got %q", strVal.ValueString())
	}
}

func TestGoValueToTerraformValue_bool(t *testing.T) {
	val, err := goValueToTerraformValue(true)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	boolVal, ok := underlying.(types.Bool)
	if !ok {
		t.Fatalf("expected Bool, got %T", underlying)
	}
	if !boolVal.ValueBool() {
		t.Fatal("expected true")
	}
}

func TestGoValueToTerraformValue_float64(t *testing.T) {
	val, err := goValueToTerraformValue(3.14)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	numVal, ok := underlying.(types.Number)
	if !ok {
		t.Fatalf("expected Number, got %T", underlying)
	}
	f, _ := numVal.ValueBigFloat().Float64()
	if f != 3.14 {
		t.Fatalf("expected 3.14, got %f", f)
	}
}

func TestGoValueToTerraformValue_int64(t *testing.T) {
	val, err := goValueToTerraformValue(int64(42))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	numVal, ok := underlying.(types.Number)
	if !ok {
		t.Fatalf("expected Number, got %T", underlying)
	}
	expected := new(big.Float).SetInt64(42)
	if numVal.ValueBigFloat().Cmp(expected) != 0 {
		t.Fatalf("expected 42, got %s", numVal.ValueBigFloat().String())
	}
}

func TestGoValueToTerraformValue_jsonNumber(t *testing.T) {
	val, err := goValueToTerraformValue(json.Number("9007199254740993"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	numVal, ok := underlying.(types.Number)
	if !ok {
		t.Fatalf("expected Number, got %T", underlying)
	}
	expected := new(big.Float).SetInt64(9007199254740993)
	if numVal.ValueBigFloat().Cmp(expected) != 0 {
		t.Fatalf("expected 9007199254740993, got %s", numVal.ValueBigFloat().String())
	}
}

func TestGoValueToTerraformValue_nil(t *testing.T) {
	val, err := goValueToTerraformValue(nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !val.UnderlyingValue().IsNull() {
		t.Fatal("expected null value")
	}
}

func TestGoValueToTerraformValue_map(t *testing.T) {
	input := map[string]interface{}{
		"name": "test",
		"age":  float64(30),
	}
	val, err := goValueToTerraformValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	objVal, ok := underlying.(types.Object)
	if !ok {
		t.Fatalf("expected Object, got %T", underlying)
	}
	attrs := objVal.Attributes()
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(attrs))
	}
}

func TestGoValueToTerraformValue_slice(t *testing.T) {
	input := []interface{}{"a", "b", "c"}
	val, err := goValueToTerraformValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	tupleVal, ok := underlying.(types.Tuple)
	if !ok {
		t.Fatalf("expected Tuple, got %T", underlying)
	}
	if len(tupleVal.Elements()) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(tupleVal.Elements()))
	}
}

func TestGoValueToTerraformValue_emptySlice(t *testing.T) {
	input := []interface{}{}
	val, err := goValueToTerraformValue(input)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	underlying := val.UnderlyingValue()
	tupleVal, ok := underlying.(types.Tuple)
	if !ok {
		t.Fatalf("expected Tuple, got %T", underlying)
	}
	if len(tupleVal.Elements()) != 0 {
		t.Fatalf("expected 0 elements, got %d", len(tupleVal.Elements()))
	}
}

func TestGoValueToTerraformValue_unsupportedType(t *testing.T) {
	_, err := goValueToTerraformValue(complex(1, 2))
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// --- helper to build function test fixtures ---

func setupFunctionTest(t *testing.T, _ *DecodeFunction, input string) (
	context.Context, function.RunRequest, *function.RunResponse,
) {
	t.Helper()
	ctx := context.Background()

	args := function.NewArgumentsData(
		[]attr.Value{types.StringValue(input)},
	)

	req := function.RunRequest{Arguments: args}
	resp := &function.RunResponse{
		Result: function.NewResultData(types.DynamicNull()),
	}

	return ctx, req, resp
}
