package provider

import "encoding/json"

func firestoreFieldsToStringMap(fields map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range fields {
		fieldMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		sv, ok := fieldMap["stringValue"]
		if !ok {
			continue
		}
		s, ok := sv.(string)
		if !ok {
			continue
		}
		result[k] = s
	}
	return result
}

func buildFirestoreWhereClause(conditions []WhereCondition) map[string]interface{} {
	buildFilter := func(cond WhereCondition) map[string]interface{} {
		var value interface{}
		if err := json.Unmarshal([]byte(cond.Value.ValueString()), &value); err != nil {
			value = cond.Value.ValueString()
		}
		return map[string]interface{}{
			"fieldFilter": map[string]interface{}{
				"field": map[string]interface{}{"fieldPath": cond.Field.ValueString()},
				"op":    cond.Operator.ValueString(),
				"value": convertToFirestoreValue(value),
			},
		}
	}

	if len(conditions) == 1 {
		return buildFilter(conditions[0])
	}

	filters := make([]interface{}, len(conditions))
	for i, cond := range conditions {
		filters[i] = buildFilter(cond)
	}
	return map[string]interface{}{
		"compositeFilter": map[string]interface{}{
			"op":      "AND",
			"filters": filters,
		},
	}
}
