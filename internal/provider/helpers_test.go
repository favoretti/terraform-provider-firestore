package provider

import (
	"encoding/json"
	"testing"
)

// Verifies extractDocumentID correctly extracts the last path segment from
// full Firestore resource names, subcollection paths, bare IDs, and empty strings.
func TestExtractDocumentID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full resource name",
			input:    "projects/my-project/databases/(default)/documents/users/abc123",
			expected: "abc123",
		},
		{
			name:     "subcollection document",
			input:    "projects/my-project/databases/(default)/documents/users/abc123/orders/order456",
			expected: "order456",
		},
		{
			name:     "just an id",
			input:    "doc1",
			expected: "doc1",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractDocumentID(tc.input)
			if got != tc.expected {
				t.Errorf("extractDocumentID(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// Verifies convertToFirestoreValue wraps scalar Go types (nil, bool, int-like
// float64, decimal float64, string, empty string) into the correct Firestore
// value envelope (nullValue, booleanValue, integerValue, doubleValue, stringValue).
func TestConvertToFirestoreValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: map[string]interface{}{"nullValue": nil},
		},
		{
			name:     "boolean true",
			input:    true,
			expected: map[string]interface{}{"booleanValue": true},
		},
		{
			name:     "boolean false",
			input:    false,
			expected: map[string]interface{}{"booleanValue": false},
		},
		{
			name:     "integer as float64",
			input:    float64(42),
			expected: map[string]interface{}{"integerValue": "42"},
		},
		{
			name:     "float64 decimal",
			input:    float64(3.14),
			expected: map[string]interface{}{"doubleValue": 3.14},
		},
		{
			name:     "string value",
			input:    "hello",
			expected: map[string]interface{}{"stringValue": "hello"},
		},
		{
			name:  "empty string",
			input: "",
			expected: map[string]interface{}{"stringValue": ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := convertToFirestoreValue(tc.input)
			gotMap, ok := got.(map[string]interface{})
			if !ok {
				t.Fatalf("expected map[string]interface{}, got %T", got)
			}
			gotJSON, _ := json.Marshal(gotMap)
			expectedJSON, _ := json.Marshal(tc.expected)
			if string(gotJSON) != string(expectedJSON) {
				t.Errorf("convertToFirestoreValue(%v) = %s, want %s", tc.input, gotJSON, expectedJSON)
			}
		})
	}
}

// Verifies convertToFirestoreValue converts a Go slice of mixed types (string,
// integer, boolean) into a Firestore arrayValue with correctly typed elements.
func TestConvertToFirestoreValue_Array(t *testing.T) {
	input := []interface{}{"a", float64(1), true}
	got := convertToFirestoreValue(input)
	gotMap := got.(map[string]interface{})

	av, ok := gotMap["arrayValue"]
	if !ok {
		t.Fatal("expected arrayValue key")
	}
	avMap := av.(map[string]interface{})
	values := avMap["values"].([]interface{})

	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}

	// First element: string
	v0 := values[0].(map[string]interface{})
	if v0["stringValue"] != "a" {
		t.Errorf("values[0] = %v, want stringValue=a", v0)
	}

	// Second element: integer
	v1 := values[1].(map[string]interface{})
	if v1["integerValue"] != "1" {
		t.Errorf("values[1] = %v, want integerValue=1", v1)
	}

	// Third element: boolean
	v2 := values[2].(map[string]interface{})
	if v2["booleanValue"] != true {
		t.Errorf("values[2] = %v, want booleanValue=true", v2)
	}
}

// Verifies convertToFirestoreValue converts a Go map into a Firestore mapValue
// with nested fields correctly wrapped in their respective type envelopes.
func TestConvertToFirestoreValue_Map(t *testing.T) {
	input := map[string]interface{}{
		"name": "test",
		"age":  float64(30),
	}
	got := convertToFirestoreValue(input)
	gotMap := got.(map[string]interface{})

	mv, ok := gotMap["mapValue"]
	if !ok {
		t.Fatal("expected mapValue key")
	}
	mvMap := mv.(map[string]interface{})
	fields := mvMap["fields"].(map[string]interface{})

	nameField := fields["name"].(map[string]interface{})
	if nameField["stringValue"] != "test" {
		t.Errorf("fields[name] = %v, want stringValue=test", nameField)
	}

	ageField := fields["age"].(map[string]interface{})
	if ageField["integerValue"] != "30" {
		t.Errorf("fields[age] = %v, want integerValue=30", ageField)
	}
}

// Verifies convertFromFirestoreValue unwraps all Firestore scalar value types
// (null, boolean, integer, double, string, reference, timestamp, bytes) back to
// native Go types, and that non-map inputs pass through unchanged.
func TestConvertFromFirestoreValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "null value",
			input:    map[string]interface{}{"nullValue": nil},
			expected: nil,
		},
		{
			name:     "boolean value",
			input:    map[string]interface{}{"booleanValue": true},
			expected: true,
		},
		{
			name:     "integer value as string",
			input:    map[string]interface{}{"integerValue": "42"},
			expected: int64(42),
		},
		{
			name:     "double value",
			input:    map[string]interface{}{"doubleValue": 3.14},
			expected: 3.14,
		},
		{
			name:     "string value",
			input:    map[string]interface{}{"stringValue": "hello"},
			expected: "hello",
		},
		{
			name:     "reference value",
			input:    map[string]interface{}{"referenceValue": "projects/p/databases/d/documents/c/doc1"},
			expected: "projects/p/databases/d/documents/c/doc1",
		},
		{
			name:     "timestamp value",
			input:    map[string]interface{}{"timestampValue": "2024-01-01T00:00:00Z"},
			expected: "2024-01-01T00:00:00Z",
		},
		{
			name:     "bytes value",
			input:    map[string]interface{}{"bytesValue": "dGVzdA=="},
			expected: "dGVzdA==",
		},
		{
			name:     "non-map input passes through",
			input:    "raw-string",
			expected: "raw-string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := convertFromFirestoreValue(tc.input)
			if got != tc.expected {
				t.Errorf("convertFromFirestoreValue(%v) = %v (%T), want %v (%T)",
					tc.input, got, got, tc.expected, tc.expected)
			}
		})
	}
}

// Verifies convertFromFirestoreValue unwraps a Firestore arrayValue containing
// mixed types into a Go slice with correctly typed elements.
func TestConvertFromFirestoreValue_Array(t *testing.T) {
	input := map[string]interface{}{
		"arrayValue": map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{"stringValue": "a"},
				map[string]interface{}{"integerValue": "1"},
			},
		},
	}

	got := convertFromFirestoreValue(input)
	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", got)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if arr[0] != "a" {
		t.Errorf("arr[0] = %v, want a", arr[0])
	}
	if arr[1] != int64(1) {
		t.Errorf("arr[1] = %v, want 1", arr[1])
	}
}

// Verifies convertFromFirestoreValue returns an empty Go slice for a Firestore
// arrayValue with no "values" key.
func TestConvertFromFirestoreValue_EmptyArray(t *testing.T) {
	input := map[string]interface{}{
		"arrayValue": map[string]interface{}{},
	}

	got := convertFromFirestoreValue(input)
	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", got)
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got %v", arr)
	}
}

// Verifies convertFromFirestoreValue unwraps a Firestore mapValue with nested
// fields into a Go map with correctly typed values.
func TestConvertFromFirestoreValue_Map(t *testing.T) {
	input := map[string]interface{}{
		"mapValue": map[string]interface{}{
			"fields": map[string]interface{}{
				"name": map[string]interface{}{"stringValue": "test"},
				"age":  map[string]interface{}{"integerValue": "30"},
			},
		},
	}

	got := convertFromFirestoreValue(input)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if m["name"] != "test" {
		t.Errorf("m[name] = %v, want test", m["name"])
	}
	if m["age"] != int64(30) {
		t.Errorf("m[age] = %v, want 30", m["age"])
	}
}

// Verifies convertFromFirestoreValue returns an empty Go map for a Firestore
// mapValue with no "fields" key.
func TestConvertFromFirestoreValue_EmptyMap(t *testing.T) {
	input := map[string]interface{}{
		"mapValue": map[string]interface{}{},
	}

	got := convertFromFirestoreValue(input)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

// Verifies convertFromFirestoreValue passes through a Firestore geoPointValue
// as its underlying map with latitude and longitude fields preserved.
func TestConvertFromFirestoreValue_GeoPoint(t *testing.T) {
	geo := map[string]interface{}{"latitude": 37.7749, "longitude": -122.4194}
	input := map[string]interface{}{"geoPointValue": geo}

	got := convertFromFirestoreValue(input)
	gotMap, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if gotMap["latitude"] != 37.7749 {
		t.Errorf("latitude = %v, want 37.7749", gotMap["latitude"])
	}
}

// Verifies jsonToFirestoreFields parses a JSON string and converts each
// top-level field into the correct Firestore value envelope.
func TestJsonToFirestoreFields(t *testing.T) {
	jsonStr := `{"name":"Alice","age":30,"active":true}`
	fields, err := jsonToFirestoreFields(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check name field
	nameField := fields["name"].(map[string]interface{})
	if nameField["stringValue"] != "Alice" {
		t.Errorf("name = %v, want stringValue=Alice", nameField)
	}

	// Check age field
	ageField := fields["age"].(map[string]interface{})
	if ageField["integerValue"] != "30" {
		t.Errorf("age = %v, want integerValue=30", ageField)
	}

	// Check active field
	activeField := fields["active"].(map[string]interface{})
	if activeField["booleanValue"] != true {
		t.Errorf("active = %v, want booleanValue=true", activeField)
	}
}

// Verifies jsonToFirestoreFields returns an error when given invalid JSON input.
func TestJsonToFirestoreFields_InvalidJSON(t *testing.T) {
	_, err := jsonToFirestoreFields("not-json")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// Verifies firestoreFieldsToJSON converts Firestore-formatted fields back into
// a valid JSON string with native Go types (string, float64 for numbers, bool).
func TestFirestoreFieldsToJSON(t *testing.T) {
	fields := map[string]interface{}{
		"name":   map[string]interface{}{"stringValue": "Alice"},
		"age":    map[string]interface{}{"integerValue": "30"},
		"active": map[string]interface{}{"booleanValue": true},
	}

	jsonStr, err := firestoreFieldsToJSON(fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if result["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", result["name"])
	}
	// JSON numbers are float64
	if result["age"] != float64(30) {
		t.Errorf("age = %v (%T), want 30", result["age"], result["age"])
	}
	if result["active"] != true {
		t.Errorf("active = %v, want true", result["active"])
	}
}

// Verifies that converting JSON to Firestore fields and back produces identical
// output, covering nested objects, arrays, strings, numbers, and booleans.
func TestRoundTrip_JSONToFirestoreAndBack(t *testing.T) {
	original := `{"name":"Bob","scores":[1,2,3],"active":true,"address":{"city":"NYC","zip":"10001"}}`

	fields, err := jsonToFirestoreFields(original)
	if err != nil {
		t.Fatalf("jsonToFirestoreFields error: %v", err)
	}

	jsonStr, err := firestoreFieldsToJSON(fields)
	if err != nil {
		t.Fatalf("firestoreFieldsToJSON error: %v", err)
	}

	// Parse both and compare
	var orig, result map[string]interface{}
	json.Unmarshal([]byte(original), &orig)
	json.Unmarshal([]byte(jsonStr), &result)

	origJSON, _ := json.Marshal(orig)
	resultJSON, _ := json.Marshal(result)

	if string(origJSON) != string(resultJSON) {
		t.Errorf("round-trip mismatch:\n  original: %s\n  result:   %s", origJSON, resultJSON)
	}
}
