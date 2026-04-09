package provider

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func goValueToTerraformValue(v interface{}) (basetypes.DynamicValue, error) {
	attrVal, err := goValueToAttrValue(v)
	if err != nil {
		return types.DynamicNull(), err
	}
	return types.DynamicValue(attrVal), nil
}

func goValueToAttrValue(v interface{}) (attr.Value, error) {
	if v == nil {
		return types.StringNull(), nil
	}

	switch val := v.(type) {
	case bool:
		return types.BoolValue(val), nil
	case string:
		return types.StringValue(val), nil
	case float64:
		return types.NumberValue(big.NewFloat(val)), nil
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return types.NumberValue(new(big.Float).SetInt64(i)), nil
		}
		if f, err := val.Float64(); err == nil {
			return types.NumberValue(big.NewFloat(f)), nil
		}
		return types.StringValue(val.String()), nil
	case int64:
		return types.NumberValue(new(big.Float).SetInt64(val)), nil
	case map[string]interface{}:
		attrTypes := make(map[string]attr.Type, len(val))
		attrVals := make(map[string]attr.Value, len(val))

		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			child, err := goValueToAttrValue(val[k])
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", k, err)
			}
			attrTypes[k] = child.Type(nil)
			attrVals[k] = child
		}
		obj, diags := types.ObjectValue(attrTypes, attrVals)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to create object: %s", diags.Errors()[0].Detail())
		}
		return obj, nil
	case []interface{}:
		if len(val) == 0 {
			return types.TupleValueMust([]attr.Type{}, []attr.Value{}), nil
		}
		elemTypes := make([]attr.Type, len(val))
		elemVals := make([]attr.Value, len(val))
		for i, elem := range val {
			child, err := goValueToAttrValue(elem)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			elemTypes[i] = child.Type(nil)
			elemVals[i] = child
		}
		tuple, diags := types.TupleValue(elemTypes, elemVals)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to create tuple: %s", diags.Errors()[0].Detail())
		}
		return tuple, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}
