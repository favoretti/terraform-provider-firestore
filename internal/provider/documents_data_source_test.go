package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Verifies buildFieldFilter constructs a Firestore fieldFilter with the correct
// fieldPath, operator, and JSON-decoded string value wrapped in a stringValue envelope.
func TestBuildFieldFilter(t *testing.T) {
	cond := WhereCondition{
		Field:    types.StringValue("status"),
		Operator: types.StringValue("EQUAL"),
		Value:    types.StringValue(`"active"`),
	}

	got := buildFieldFilter(cond)

	ff, ok := got["fieldFilter"].(map[string]interface{})
	if !ok {
		t.Fatal("expected fieldFilter key")
	}

	field := ff["field"].(map[string]interface{})
	if field["fieldPath"] != "status" {
		t.Errorf("fieldPath = %v, want status", field["fieldPath"])
	}

	if ff["op"] != "EQUAL" {
		t.Errorf("op = %v, want EQUAL", ff["op"])
	}

	value := ff["value"].(map[string]interface{})
	if value["stringValue"] != "active" {
		t.Errorf("value = %v, want stringValue=active", value)
	}
}

// Verifies buildFieldFilter correctly JSON-decodes a numeric string ("18") and
// wraps it as a Firestore integerValue in the filter.
func TestBuildFieldFilter_IntegerValue(t *testing.T) {
	cond := WhereCondition{
		Field:    types.StringValue("age"),
		Operator: types.StringValue("GREATER_THAN"),
		Value:    types.StringValue("18"),
	}

	got := buildFieldFilter(cond)
	ff := got["fieldFilter"].(map[string]interface{})
	value := ff["value"].(map[string]interface{})

	if value["integerValue"] != "18" {
		t.Errorf("value = %v, want integerValue=18", value)
	}
}

// Verifies buildFieldFilter correctly JSON-decodes a boolean string ("true") and
// wraps it as a Firestore booleanValue in the filter.
func TestBuildFieldFilter_BooleanValue(t *testing.T) {
	cond := WhereCondition{
		Field:    types.StringValue("active"),
		Operator: types.StringValue("EQUAL"),
		Value:    types.StringValue("true"),
	}

	got := buildFieldFilter(cond)
	ff := got["fieldFilter"].(map[string]interface{})
	value := ff["value"].(map[string]interface{})

	if value["booleanValue"] != true {
		t.Errorf("value = %v, want booleanValue=true", value)
	}
}

// Verifies listDocuments sends a GET to the correct collection path, parses a
// response with two documents, and extracts document IDs and field values.
func TestListDocuments_Success(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path

		resp := map[string]interface{}{
			"documents": []map[string]interface{}{
				{
					"name": "projects/test-project/databases/(default)/documents/users/doc1",
					"fields": map[string]interface{}{
						"name": map[string]interface{}{"stringValue": "Alice"},
					},
					"createTime": "2024-01-01T00:00:00Z",
					"updateTime": "2024-01-01T00:00:00Z",
				},
				{
					"name": "projects/test-project/databases/(default)/documents/users/doc2",
					"fields": map[string]interface{}{
						"name": map[string]interface{}{"stringValue": "Bob"},
					},
					"createTime": "2024-01-02T00:00:00Z",
					"updateTime": "2024-01-02T00:00:00Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}
	ctx := context.Background()
	docs, diags := ds.listDocuments(ctx, "test-project", "(default)", "users")

	if diags.HasError() {
		for _, d := range diags.Errors() {
			t.Fatalf("unexpected diag: %s: %s", d.Summary(), d.Detail())
		}
	}

	if capturedMethod != "GET" {
		t.Errorf("method = %s, want GET", capturedMethod)
	}
	if capturedPath != "/v1/projects/test-project/databases/(default)/documents/users" {
		t.Errorf("path = %s, want /v1/projects/test-project/databases/(default)/documents/users", capturedPath)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if docs[0].DocumentID.ValueString() != "doc1" {
		t.Errorf("docs[0].document_id = %s, want doc1", docs[0].DocumentID.ValueString())
	}
	if docs[1].DocumentID.ValueString() != "doc2" {
		t.Errorf("docs[1].document_id = %s, want doc2", docs[1].DocumentID.ValueString())
	}

	var fields map[string]interface{}
	json.Unmarshal([]byte(docs[0].Fields.ValueString()), &fields)
	if fields["name"] != "Alice" {
		t.Errorf("docs[0].fields.name = %v, want Alice", fields["name"])
	}
}

// Verifies listDocuments returns an empty result set without errors when the
// API returns a collection with no documents.
func TestListDocuments_EmptyCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"documents": []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}
	ctx := context.Background()
	docs, diags := ds.listDocuments(ctx, "test-project", "(default)", "empty_collection")

	if diags.HasError() {
		t.Fatal("unexpected error for empty collection")
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents, got %d", len(docs))
	}
}

// Verifies listDocuments surfaces a diagnostic error when the Firestore API
// returns a 403 Forbidden response.
func TestListDocuments_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": {"message": "Permission denied"}}`))
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}
	ctx := context.Background()
	_, diags := ds.listDocuments(ctx, "test-project", "(default)", "users")

	if !diags.HasError() {
		t.Fatal("expected error for 403 response")
	}
}

// Verifies listDocuments constructs the correct URL path for subcollections
// (e.g., users/u1/orders) and extracts the document ID from the nested path.
func TestListDocuments_Subcollection(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		resp := map[string]interface{}{
			"documents": []map[string]interface{}{
				{
					"name": "projects/test-project/databases/(default)/documents/users/u1/orders/order1",
					"fields": map[string]interface{}{
						"product": map[string]interface{}{"stringValue": "Widget"},
					},
					"createTime": "2024-01-01T00:00:00Z",
					"updateTime": "2024-01-01T00:00:00Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}
	ctx := context.Background()
	docs, diags := ds.listDocuments(ctx, "test-project", "(default)", "users/u1/orders")

	if diags.HasError() {
		t.Fatal("unexpected error")
	}
	if capturedPath != "/v1/projects/test-project/databases/(default)/documents/users/u1/orders" {
		t.Errorf("path = %s, want subcollection path", capturedPath)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].DocumentID.ValueString() != "order1" {
		t.Errorf("document_id = %s, want order1", docs[0].DocumentID.ValueString())
	}
}

// Verifies runStructuredQuery with a single where condition produces a plain
// fieldFilter (not wrapped in a compositeFilter), sends the correct request
// body, and parses the response document.
func TestRunStructuredQuery_SingleFilter(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{
				"document": map[string]interface{}{
					"name": "projects/test-project/databases/(default)/documents/users/doc1",
					"fields": map[string]interface{}{
						"status": map[string]interface{}{"stringValue": "active"},
					},
					"createTime": "2024-01-01T00:00:00Z",
					"updateTime": "2024-01-01T00:00:00Z",
				},
				"readTime": "2024-01-01T00:00:00Z",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Patch the client to use the test server
	client := newTestClient(server)
	ds := &DocumentsDataSource{client: client}

	whereConditions := []WhereCondition{
		{
			Field:    types.StringValue("status"),
			Operator: types.StringValue("EQUAL"),
			Value:    types.StringValue(`"active"`),
		},
	}

	ctx := context.Background()
	// We can't easily redirect the URL, but we can test buildFieldFilter and query construction
	// by calling runStructuredQuery with a mock that captures the request body.
	// The URL won't match the test server, so let's test the query construction separately.

	// Test query construction by building the query ourselves
	var allFilters []interface{}
	for _, cond := range whereConditions {
		allFilters = append(allFilters, buildFieldFilter(cond))
	}

	if len(allFilters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(allFilters))
	}

	// With a single filter, the where clause should be the filter directly (not wrapped in compositeFilter)
	filter := allFilters[0].(map[string]interface{})
	if _, ok := filter["fieldFilter"]; !ok {
		t.Error("expected fieldFilter for single condition")
	}

	// Now test that runStructuredQuery parses the response correctly
	// by providing a server that responds regardless of path
	docs, diags := ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		whereConditions, nil, nil, types.Int64Null(), "AND")

	if diags.HasError() {
		for _, d := range diags.Errors() {
			t.Logf("diag: %s: %s", d.Summary(), d.Detail())
		}
		// The URL won't match the test server path, but the test server catches all requests
	}

	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].DocumentID.ValueString() != "doc1" {
		t.Errorf("document_id = %s, want doc1", docs[0].DocumentID.ValueString())
	}

	// Verify the query body sent to the server
	sq := capturedBody["structuredQuery"].(map[string]interface{})
	from := sq["from"].([]interface{})
	fromMap := from[0].(map[string]interface{})
	if fromMap["collectionId"] != "users" {
		t.Errorf("collectionId = %v, want users", fromMap["collectionId"])
	}

	// Single filter should be a fieldFilter, not compositeFilter
	where := sq["where"].(map[string]interface{})
	if _, ok := where["fieldFilter"]; !ok {
		t.Error("expected fieldFilter for single where condition")
	}
}

// Verifies runStructuredQuery with two where conditions and filter_operator=AND
// produces a compositeFilter with op=AND containing both field filters.
func TestRunStructuredQuery_MultipleFilters_AND(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	whereConditions := []WhereCondition{
		{
			Field:    types.StringValue("status"),
			Operator: types.StringValue("EQUAL"),
			Value:    types.StringValue(`"active"`),
		},
		{
			Field:    types.StringValue("age"),
			Operator: types.StringValue("GREATER_THAN"),
			Value:    types.StringValue("18"),
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		whereConditions, nil, nil, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	where := sq["where"].(map[string]interface{})

	cf, ok := where["compositeFilter"].(map[string]interface{})
	if !ok {
		t.Fatal("expected compositeFilter for multiple where conditions")
	}

	if cf["op"] != "AND" {
		t.Errorf("compositeFilter op = %v, want AND", cf["op"])
	}

	filters := cf["filters"].([]interface{})
	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}
}

// Verifies runStructuredQuery with two where conditions and filter_operator=OR
// produces a compositeFilter with op=OR.
func TestRunStructuredQuery_MultipleFilters_OR(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	whereConditions := []WhereCondition{
		{
			Field:    types.StringValue("role"),
			Operator: types.StringValue("EQUAL"),
			Value:    types.StringValue(`"admin"`),
		},
		{
			Field:    types.StringValue("role"),
			Operator: types.StringValue("EQUAL"),
			Value:    types.StringValue(`"editor"`),
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		whereConditions, nil, nil, types.Int64Null(), "OR")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	where := sq["where"].(map[string]interface{})
	cf := where["compositeFilter"].(map[string]interface{})

	if cf["op"] != "OR" {
		t.Errorf("compositeFilter op = %v, want OR", cf["op"])
	}
}

// Verifies runStructuredQuery with a top-level where condition AND a where_group
// produces nested composite filters: an outer AND combining a fieldFilter with
// an inner OR compositeFilter, matching the query:
// status = "active" AND (role = "admin" OR role = "editor").
func TestRunStructuredQuery_WhereGroupNestedFilters(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	// status = "active" AND (role = "admin" OR role = "editor")
	whereConditions := []WhereCondition{
		{
			Field:    types.StringValue("status"),
			Operator: types.StringValue("EQUAL"),
			Value:    types.StringValue(`"active"`),
		},
	}

	// Build a where_group with OR conditions using types.List
	groupWhereType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"field":    types.StringType,
			"operator": types.StringType,
			"value":    types.StringType,
		},
	}

	groupWhereList := types.ListValueMust(groupWhereType, []attr.Value{
		types.ObjectValueMust(
			map[string]attr.Type{
				"field":    types.StringType,
				"operator": types.StringType,
				"value":    types.StringType,
			},
			map[string]attr.Value{
				"field":    types.StringValue("role"),
				"operator": types.StringValue("EQUAL"),
				"value":    types.StringValue(`"admin"`),
			},
		),
		types.ObjectValueMust(
			map[string]attr.Type{
				"field":    types.StringType,
				"operator": types.StringType,
				"value":    types.StringType,
			},
			map[string]attr.Value{
				"field":    types.StringValue("role"),
				"operator": types.StringValue("EQUAL"),
				"value":    types.StringValue(`"editor"`),
			},
		),
	})

	whereGroups := []WhereGroupCondition{
		{
			GroupOperator: types.StringValue("OR"),
			Where:         groupWhereList,
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		whereConditions, whereGroups, nil, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	where := sq["where"].(map[string]interface{})

	// Top-level should be AND composite filter
	cf, ok := where["compositeFilter"].(map[string]interface{})
	if !ok {
		t.Fatal("expected top-level compositeFilter")
	}

	if cf["op"] != "AND" {
		t.Errorf("top-level compositeFilter op = %v, want AND", cf["op"])
	}

	filters := cf["filters"].([]interface{})
	if len(filters) != 2 {
		t.Fatalf("expected 2 top-level filters, got %d", len(filters))
	}

	// First filter: fieldFilter for status=active
	f0 := filters[0].(map[string]interface{})
	if _, ok := f0["fieldFilter"]; !ok {
		t.Error("first filter should be a fieldFilter")
	}

	// Second filter: nested compositeFilter with OR
	f1 := filters[1].(map[string]interface{})
	nestedCF, ok := f1["compositeFilter"].(map[string]interface{})
	if !ok {
		t.Fatal("second filter should be a compositeFilter (the where_group)")
	}

	if nestedCF["op"] != "OR" {
		t.Errorf("nested compositeFilter op = %v, want OR", nestedCF["op"])
	}

	nestedFilters := nestedCF["filters"].([]interface{})
	if len(nestedFilters) != 2 {
		t.Fatalf("expected 2 nested filters, got %d", len(nestedFilters))
	}

	// Verify nested filter field values
	nf0 := nestedFilters[0].(map[string]interface{})
	nf0ff := nf0["fieldFilter"].(map[string]interface{})
	nf0field := nf0ff["field"].(map[string]interface{})
	if nf0field["fieldPath"] != "role" {
		t.Errorf("nested filter 0 fieldPath = %v, want role", nf0field["fieldPath"])
	}
	nf0val := nf0ff["value"].(map[string]interface{})
	if nf0val["stringValue"] != "admin" {
		t.Errorf("nested filter 0 value = %v, want admin", nf0val["stringValue"])
	}

	nf1 := nestedFilters[1].(map[string]interface{})
	nf1ff := nf1["fieldFilter"].(map[string]interface{})
	nf1val := nf1ff["value"].(map[string]interface{})
	if nf1val["stringValue"] != "editor" {
		t.Errorf("nested filter 1 value = %v, want editor", nf1val["stringValue"])
	}
}

// Verifies runStructuredQuery includes orderBy entries with the correct field
// paths and explicit directions (ASCENDING, DESCENDING) in the request body.
func TestRunStructuredQuery_OrderBy(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	orderBy := []OrderByCondition{
		{
			Field:     types.StringValue("name"),
			Direction: types.StringValue("ASCENDING"),
		},
		{
			Field:     types.StringValue("age"),
			Direction: types.StringValue("DESCENDING"),
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, nil, orderBy, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	ob := sq["orderBy"].([]interface{})

	if len(ob) != 2 {
		t.Fatalf("expected 2 orderBy entries, got %d", len(ob))
	}

	ob0 := ob[0].(map[string]interface{})
	ob0field := ob0["field"].(map[string]interface{})
	if ob0field["fieldPath"] != "name" {
		t.Errorf("orderBy[0] fieldPath = %v, want name", ob0field["fieldPath"])
	}
	if ob0["direction"] != "ASCENDING" {
		t.Errorf("orderBy[0] direction = %v, want ASCENDING", ob0["direction"])
	}

	ob1 := ob[1].(map[string]interface{})
	if ob1["direction"] != "DESCENDING" {
		t.Errorf("orderBy[1] direction = %v, want DESCENDING", ob1["direction"])
	}
}

// Verifies runStructuredQuery defaults the orderBy direction to ASCENDING when
// the direction attribute is null/unset.
func TestRunStructuredQuery_OrderByDefaultDirection(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	orderBy := []OrderByCondition{
		{
			Field:     types.StringValue("name"),
			Direction: types.StringNull(),
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, nil, orderBy, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	ob := sq["orderBy"].([]interface{})
	ob0 := ob[0].(map[string]interface{})
	if ob0["direction"] != "ASCENDING" {
		t.Errorf("default direction = %v, want ASCENDING", ob0["direction"])
	}
}

// Verifies runStructuredQuery includes a numeric limit field in the request body
// when the limit attribute is set.
func TestRunStructuredQuery_WithLimit(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, nil, nil, types.Int64Value(10), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})

	// limit is serialized as a JSON number
	limit, ok := sq["limit"].(float64)
	if !ok {
		t.Fatalf("expected limit to be a number, got %T: %v", sq["limit"], sq["limit"])
	}
	if limit != 10 {
		t.Errorf("limit = %v, want 10", limit)
	}
}

// Verifies runStructuredQuery omits where, orderBy, and limit from the request
// body when none are specified, producing a minimal query with only the
// collection "from" clause.
func TestRunStructuredQuery_NoFilters(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, nil, nil, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})

	if _, ok := sq["where"]; ok {
		t.Error("expected no where clause when no filters provided")
	}
	if _, ok := sq["orderBy"]; ok {
		t.Error("expected no orderBy clause when no order_by provided")
	}
	if _, ok := sq["limit"]; ok {
		t.Error("expected no limit when not provided")
	}
}

// Verifies runStructuredQuery correctly parses a multi-document response,
// skipping results with null documents (e.g., skippedResults entries), and
// extracts document IDs, timestamps, and field values from each result.
func TestRunStructuredQuery_ResponseParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []map[string]interface{}{
			{
				"document": map[string]interface{}{
					"name": "projects/test-project/databases/(default)/documents/users/alice",
					"fields": map[string]interface{}{
						"name":   map[string]interface{}{"stringValue": "Alice"},
						"age":    map[string]interface{}{"integerValue": "30"},
						"active": map[string]interface{}{"booleanValue": true},
					},
					"createTime": "2024-01-01T00:00:00Z",
					"updateTime": "2024-06-15T12:00:00Z",
				},
				"readTime": "2024-06-15T12:00:00Z",
			},
			{
				"document": map[string]interface{}{
					"name": "projects/test-project/databases/(default)/documents/users/bob",
					"fields": map[string]interface{}{
						"name": map[string]interface{}{"stringValue": "Bob"},
					},
					"createTime": "2024-02-01T00:00:00Z",
					"updateTime": "2024-06-15T12:00:00Z",
				},
				"readTime": "2024-06-15T12:00:00Z",
			},
			{
				// Result with no document (e.g., skipped)
				"readTime": "2024-06-15T12:00:00Z",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	ctx := context.Background()
	docs, diags := ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, nil, nil, types.Int64Null(), "AND")

	if diags.HasError() {
		for _, d := range diags.Errors() {
			t.Fatalf("unexpected diag error: %s: %s", d.Summary(), d.Detail())
		}
	}

	// Should have 2 documents (third result had no document)
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}

	if docs[0].DocumentID.ValueString() != "alice" {
		t.Errorf("docs[0].document_id = %s, want alice", docs[0].DocumentID.ValueString())
	}
	if docs[0].CreateTime.ValueString() != "2024-01-01T00:00:00Z" {
		t.Errorf("docs[0].create_time = %s, want 2024-01-01T00:00:00Z", docs[0].CreateTime.ValueString())
	}

	// Verify fields JSON is valid and contains expected values
	var fields0 map[string]interface{}
	json.Unmarshal([]byte(docs[0].Fields.ValueString()), &fields0)
	if fields0["name"] != "Alice" {
		t.Errorf("docs[0].fields.name = %v, want Alice", fields0["name"])
	}

	if docs[1].DocumentID.ValueString() != "bob" {
		t.Errorf("docs[1].document_id = %s, want bob", docs[1].DocumentID.ValueString())
	}
}

// Verifies runStructuredQuery surfaces a diagnostic error when the Firestore
// API returns a 400 Bad Request response.
func TestRunStructuredQuery_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid query"}}`))
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	ctx := context.Background()
	_, diags := ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, nil, nil, types.Int64Null(), "AND")

	if !diags.HasError() {
		t.Fatal("expected error for 400 response")
	}
}

// Verifies that a where_group containing only a single condition is flattened
// to a plain fieldFilter rather than being wrapped in an unnecessary
// compositeFilter, since Firestore doesn't require composite wrapping for one filter.
func TestRunStructuredQuery_WhereGroupSingleCondition(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	groupWhereType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"field":    types.StringType,
			"operator": types.StringType,
			"value":    types.StringType,
		},
	}

	// A where_group with a single condition should produce a fieldFilter, not a compositeFilter
	groupWhereList := types.ListValueMust(groupWhereType, []attr.Value{
		types.ObjectValueMust(
			map[string]attr.Type{
				"field":    types.StringType,
				"operator": types.StringType,
				"value":    types.StringType,
			},
			map[string]attr.Value{
				"field":    types.StringValue("role"),
				"operator": types.StringValue("EQUAL"),
				"value":    types.StringValue(`"admin"`),
			},
		),
	})

	whereGroups := []WhereGroupCondition{
		{
			GroupOperator: types.StringValue("OR"),
			Where:         groupWhereList,
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, whereGroups, nil, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	where := sq["where"].(map[string]interface{})

	// Single condition in a single group = plain fieldFilter (no compositeFilter wrapping)
	if _, ok := where["fieldFilter"]; !ok {
		t.Error("expected fieldFilter for single condition in where_group, got compositeFilter")
	}
}

// Verifies that multiple where_group blocks are combined at the top level using
// the filter_operator. The first group (2 conditions) becomes a compositeFilter
// while the second group (1 condition) is flattened to a fieldFilter.
func TestRunStructuredQuery_MultipleWhereGroups(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	groupWhereType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"field":    types.StringType,
			"operator": types.StringType,
			"value":    types.StringType,
		},
	}

	makeGroupWhere := func(field, op, val string) types.List {
		return types.ListValueMust(groupWhereType, []attr.Value{
			types.ObjectValueMust(
				map[string]attr.Type{
					"field":    types.StringType,
					"operator": types.StringType,
					"value":    types.StringType,
				},
				map[string]attr.Value{
					"field":    types.StringValue(field),
					"operator": types.StringValue(op),
					"value":    types.StringValue(val),
				},
			),
		})
	}

	// Two where_groups combined with AND at top level
	whereGroups := []WhereGroupCondition{
		{
			GroupOperator: types.StringValue("OR"),
			Where: types.ListValueMust(groupWhereType, []attr.Value{
				types.ObjectValueMust(
					map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType},
					map[string]attr.Value{"field": types.StringValue("role"), "operator": types.StringValue("EQUAL"), "value": types.StringValue(`"admin"`)},
				),
				types.ObjectValueMust(
					map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType},
					map[string]attr.Value{"field": types.StringValue("role"), "operator": types.StringValue("EQUAL"), "value": types.StringValue(`"editor"`)},
				),
			}),
		},
		{
			GroupOperator: types.StringValue("OR"),
			Where:         makeGroupWhere("dept", "EQUAL", `"engineering"`),
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, whereGroups, nil, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	where := sq["where"].(map[string]interface{})

	// Multiple groups = top-level AND compositeFilter
	cf := where["compositeFilter"].(map[string]interface{})
	if cf["op"] != "AND" {
		t.Errorf("top-level op = %v, want AND", cf["op"])
	}

	filters := cf["filters"].([]interface{})
	if len(filters) != 2 {
		t.Fatalf("expected 2 top-level filters, got %d", len(filters))
	}

	// First should be a compositeFilter (OR with 2 conditions)
	f0 := filters[0].(map[string]interface{})
	if _, ok := f0["compositeFilter"]; !ok {
		t.Error("first filter should be a compositeFilter")
	}

	// Second should be a fieldFilter (single condition in group)
	f1 := filters[1].(map[string]interface{})
	if _, ok := f1["fieldFilter"]; !ok {
		t.Error("second filter should be a fieldFilter (single condition)")
	}
}

// Verifies that a where_group with a null/unset group_operator defaults to AND
// when combining its inner conditions.
func TestRunStructuredQuery_WhereGroupDefaultOperator(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := []map[string]interface{}{
			{"readTime": "2024-01-01T00:00:00Z"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ds := &DocumentsDataSource{client: newTestClient(server)}

	groupWhereType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"field":    types.StringType,
			"operator": types.StringType,
			"value":    types.StringType,
		},
	}

	// where_group with no group_operator should default to AND
	whereGroups := []WhereGroupCondition{
		{
			GroupOperator: types.StringNull(),
			Where: types.ListValueMust(groupWhereType, []attr.Value{
				types.ObjectValueMust(
					map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType},
					map[string]attr.Value{"field": types.StringValue("a"), "operator": types.StringValue("EQUAL"), "value": types.StringValue(`"1"`)},
				),
				types.ObjectValueMust(
					map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType},
					map[string]attr.Value{"field": types.StringValue("b"), "operator": types.StringValue("EQUAL"), "value": types.StringValue(`"2"`)},
				),
			}),
		},
	}

	ctx := context.Background()
	ds.runStructuredQuery(ctx, "test-project", "(default)", "users",
		nil, whereGroups, nil, types.Int64Null(), "AND")

	sq := capturedBody["structuredQuery"].(map[string]interface{})
	where := sq["where"].(map[string]interface{})
	cf := where["compositeFilter"].(map[string]interface{})

	if cf["op"] != "AND" {
		t.Errorf("default group_operator = %v, want AND", cf["op"])
	}
}
