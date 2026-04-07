package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFirestoreFieldsToStringMap_onlyStrings(t *testing.T) {
	fields := map[string]interface{}{
		"name":   map[string]interface{}{"stringValue": "alice"},
		"age":    map[string]interface{}{"integerValue": "30"},
		"active": map[string]interface{}{"booleanValue": true},
		"meta":   map[string]interface{}{"mapValue": map[string]interface{}{}},
	}

	result := firestoreFieldsToStringMap(fields)

	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d: %v", len(result), result)
	}
	if result["name"] != "alice" {
		t.Errorf("expected name=alice, got %q", result["name"])
	}
}

func TestFirestoreFieldsToStringMap_empty(t *testing.T) {
	result := firestoreFieldsToStringMap(map[string]interface{}{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestBuildFirestoreWhereClause_single(t *testing.T) {
	conditions := []WhereCondition{
		{
			Field:    types.StringValue("status"),
			Operator: types.StringValue("EQUAL"),
			Value:    types.StringValue("active"),
		},
	}

	result := buildFirestoreWhereClause(conditions)

	if _, ok := result["fieldFilter"]; !ok {
		t.Errorf("expected fieldFilter key for single condition, got keys: %v", keysOf(result))
	}
	if _, ok := result["compositeFilter"]; ok {
		t.Error("single condition must not produce compositeFilter")
	}
}

func TestBuildFirestoreWhereClause_multiple(t *testing.T) {
	conditions := []WhereCondition{
		{Field: types.StringValue("status"), Operator: types.StringValue("EQUAL"), Value: types.StringValue("active")},
		{Field: types.StringValue("role"), Operator: types.StringValue("EQUAL"), Value: types.StringValue("admin")},
	}

	result := buildFirestoreWhereClause(conditions)

	cf, ok := result["compositeFilter"]
	if !ok {
		t.Fatalf("expected compositeFilter for multiple conditions, got keys: %v", keysOf(result))
	}
	cfMap, ok := cf.(map[string]interface{})
	if !ok {
		t.Fatalf("compositeFilter is not a map")
	}
	if cfMap["op"] != "AND" {
		t.Errorf("expected op=AND, got %v", cfMap["op"])
	}
	filters, ok := cfMap["filters"].([]interface{})
	if !ok {
		t.Fatalf("filters is not a slice")
	}
	if len(filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(filters))
	}
}

func TestExtractDocumentID(t *testing.T) {
	name := "projects/my-project/databases/(default)/documents/users/abc123"
	got := extractDocumentID(name)
	if got != "abc123" {
		t.Errorf("expected abc123, got %q", got)
	}
}

func TestConvertToFirestoreValue_string(t *testing.T) {
	result := convertToFirestoreValue("hello")
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["stringValue"] != "hello" {
		t.Errorf("expected stringValue=hello, got %v", m)
	}
}

func TestConvertToFirestoreValue_bool(t *testing.T) {
	result := convertToFirestoreValue(true)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["booleanValue"] != true {
		t.Errorf("expected booleanValue=true, got %v", m)
	}
}

func TestConvertToFirestoreValue_roundTrip(t *testing.T) {
	original := map[string]interface{}{
		"name":   "alice",
		"active": true,
	}

	firestoreFields := map[string]interface{}{}
	for k, v := range original {
		firestoreFields[k] = convertToFirestoreValue(v)
	}

	recovered := convertFromFirestoreFields(firestoreFields)

	if recovered["name"] != "alice" {
		t.Errorf("name round-trip failed: got %v", recovered["name"])
	}
	if recovered["active"] != true {
		t.Errorf("active round-trip failed: got %v", recovered["active"])
	}
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
