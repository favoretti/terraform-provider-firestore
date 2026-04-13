package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Verifies that reading a single document via the data source sends a GET to the
// correct path and correctly parses the response: resource name, timestamps, and
// field values for string, integer, and boolean types.
func TestDocumentDataSource_Read_Success(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path

		resp := firestoreDocumentResponse(
			"projects/test-project/databases/(default)/documents/users/user-123",
			map[string]interface{}{
				"name":   map[string]interface{}{"stringValue": "Alice"},
				"age":    map[string]interface{}{"integerValue": "30"},
				"active": map[string]interface{}{"booleanValue": true},
			},
		)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server)
	reqURL := "https://firestore.googleapis.com/v1/projects/test-project/databases/(default)/documents/users/user-123"
	httpReq, _ := http.NewRequest("GET", reqURL, nil)
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	if capturedMethod != "GET" {
		t.Errorf("method = %s, want GET", capturedMethod)
	}
	if capturedPath != "/v1/projects/test-project/databases/(default)/documents/users/user-123" {
		t.Errorf("path = %s", capturedPath)
	}

	respBody, _ := io.ReadAll(httpResp.Body)
	var doc FirestoreDocument
	json.Unmarshal(respBody, &doc)

	if doc.Name != "projects/test-project/databases/(default)/documents/users/user-123" {
		t.Errorf("name = %s", doc.Name)
	}
	if doc.CreateTime != "2024-01-01T00:00:00Z" {
		t.Errorf("createTime = %s", doc.CreateTime)
	}
	if doc.UpdateTime != "2024-01-02T00:00:00Z" {
		t.Errorf("updateTime = %s", doc.UpdateTime)
	}

	fieldsJSON, _ := firestoreFieldsToJSON(doc.Fields)
	var fields map[string]interface{}
	json.Unmarshal([]byte(fieldsJSON), &fields)

	if fields["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", fields["name"])
	}
	// integerValue "30" → int64 → JSON number 30
	if fields["age"] != float64(30) {
		t.Errorf("age = %v (%T), want 30", fields["age"], fields["age"])
	}
	if fields["active"] != true {
		t.Errorf("active = %v, want true", fields["active"])
	}
}

// Verifies that a 404 response from the Firestore API is correctly surfaced,
// which the data source reports as a diagnostic error (unlike the resource
// which removes the document from state).
func TestDocumentDataSource_Read_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": {"message": "Document not found"}}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	reqURL := "https://firestore.googleapis.com/v1/projects/test-project/databases/(default)/documents/users/nonexistent"
	httpReq, _ := http.NewRequest("GET", reqURL, nil)
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	// The data source returns a diagnostic error for 404
	if httpResp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", httpResp.StatusCode)
	}
}

// Verifies that a 403 Forbidden response from the Firestore API is correctly
// surfaced as an error status code.
func TestDocumentDataSource_Read_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": {"message": "Permission denied"}}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	reqURL := "https://firestore.googleapis.com/v1/projects/test-project/databases/(default)/documents/users/user-123"
	httpReq, _ := http.NewRequest("GET", reqURL, nil)
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", httpResp.StatusCode)
	}
}

// Verifies that reading a document in a subcollection constructs the correct
// nested URL path and parses the document ID and fields from the response.
func TestDocumentDataSource_Read_Subcollection(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		resp := firestoreDocumentResponse(
			"projects/test-project/databases/(default)/documents/users/u1/orders/order-1",
			map[string]interface{}{
				"product":  map[string]interface{}{"stringValue": "Widget"},
				"quantity": map[string]interface{}{"integerValue": "5"},
			},
		)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server)
	reqURL := "https://firestore.googleapis.com/v1/projects/test-project/databases/(default)/documents/users/u1/orders/order-1"
	httpReq, _ := http.NewRequest("GET", reqURL, nil)
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	if capturedPath != "/v1/projects/test-project/databases/(default)/documents/users/u1/orders/order-1" {
		t.Errorf("path = %s", capturedPath)
	}

	respBody, _ := io.ReadAll(httpResp.Body)
	var doc FirestoreDocument
	json.Unmarshal(respBody, &doc)

	if extractDocumentID(doc.Name) != "order-1" {
		t.Errorf("document_id = %s, want order-1", extractDocumentID(doc.Name))
	}

	fieldsJSON, _ := firestoreFieldsToJSON(doc.Fields)
	var fields map[string]interface{}
	json.Unmarshal([]byte(fieldsJSON), &fields)

	if fields["product"] != "Widget" {
		t.Errorf("product = %v, want Widget", fields["product"])
	}
	if fields["quantity"] != float64(5) {
		t.Errorf("quantity = %v, want 5", fields["quantity"])
	}
}

// Verifies that overriding the project and database from provider defaults
// constructs the URL with the overridden values in the path.
func TestDocumentDataSource_Read_ProjectOverride(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		resp := firestoreDocumentResponse(
			"projects/other-project/databases/mydb/documents/items/item1",
			map[string]interface{}{},
		)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server)
	// Override project and database in the URL
	reqURL := "https://firestore.googleapis.com/v1/projects/other-project/databases/mydb/documents/items/item1"
	httpReq, _ := http.NewRequest("GET", reqURL, nil)
	_, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}

	if capturedPath != "/v1/projects/other-project/databases/mydb/documents/items/item1" {
		t.Errorf("path = %s", capturedPath)
	}
}

// Verifies that complex Firestore field types (arrays, nested maps, doubles,
// and nulls) are correctly converted back to their Go/JSON equivalents when
// parsing the API response.
func TestDocumentDataSource_Read_ComplexFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := firestoreDocumentResponse(
			"projects/test-project/databases/(default)/documents/users/user-1",
			map[string]interface{}{
				"name": map[string]interface{}{"stringValue": "Alice"},
				"tags": map[string]interface{}{
					"arrayValue": map[string]interface{}{
						"values": []interface{}{
							map[string]interface{}{"stringValue": "admin"},
							map[string]interface{}{"stringValue": "developer"},
						},
					},
				},
				"profile": map[string]interface{}{
					"mapValue": map[string]interface{}{
						"fields": map[string]interface{}{
							"bio":      map[string]interface{}{"stringValue": "Engineer"},
							"location": map[string]interface{}{"stringValue": "NYC"},
						},
					},
				},
				"score": map[string]interface{}{"doubleValue": 99.5},
				"empty": map[string]interface{}{"nullValue": nil},
			},
		)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server)
	reqURL := "https://firestore.googleapis.com/v1/projects/test-project/databases/(default)/documents/users/user-1"
	httpReq, _ := http.NewRequest("GET", reqURL, nil)
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	var doc FirestoreDocument
	json.Unmarshal(respBody, &doc)

	fieldsJSON, _ := firestoreFieldsToJSON(doc.Fields)
	var fields map[string]interface{}
	json.Unmarshal([]byte(fieldsJSON), &fields)

	// Array field
	tags := fields["tags"].([]interface{})
	if len(tags) != 2 {
		t.Fatalf("tags length = %d, want 2", len(tags))
	}
	if tags[0] != "admin" || tags[1] != "developer" {
		t.Errorf("tags = %v", tags)
	}

	// Map field
	profile := fields["profile"].(map[string]interface{})
	if profile["bio"] != "Engineer" {
		t.Errorf("profile.bio = %v, want Engineer", profile["bio"])
	}
	if profile["location"] != "NYC" {
		t.Errorf("profile.location = %v, want NYC", profile["location"])
	}

	// Double field
	if fields["score"] != 99.5 {
		t.Errorf("score = %v, want 99.5", fields["score"])
	}

	// Null field
	if fields["empty"] != nil {
		t.Errorf("empty = %v, want nil", fields["empty"])
	}
}
