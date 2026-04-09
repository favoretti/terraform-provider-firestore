package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestDocumentsDataSourceSchema_selectAttribute verifies the select attribute
// exists and is Optional (failure mode 4: missing input validation).
func TestDocumentsDataSourceSchema_selectAttribute(t *testing.T) {
	ctx := context.Background()
	ds := NewDocumentsDataSource()
	schemaResp := datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	selectAttr, ok := schemaResp.Schema.Attributes["select"]
	if !ok {
		t.Fatal("select attribute missing from schema")
	}

	listAttr, ok := selectAttr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("select should be ListAttribute, got %T", selectAttr)
	}

	if !listAttr.Optional {
		t.Error("select should be Optional")
	}
}

func TestAccDocumentsDataSource_list(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "a" {
  collection  = "tf-acc-test-list"
  document_id = "list-doc-a"
  fields      = jsonencode({ name = "a" })
}
resource "firestore_document" "b" {
  collection  = "tf-acc-test-list"
  document_id = "list-doc-b"
  fields      = jsonencode({ name = "b" })
}
data "firestore_documents" "test" {
  collection = "tf-acc-test-list"
  depends_on = [firestore_document.a, firestore_document.b]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents_map").AtMapKey("list-doc-a").AtMapKey("document_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents_map").AtMapKey("list-doc-b").AtMapKey("document_id"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}

func TestAccDocumentsDataSource_whereFilter(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "match" {
  collection  = "tf-acc-test-where"
  document_id = "where-match"
  fields      = jsonencode({ env = "test" })
}
resource "firestore_document" "nomatch" {
  collection  = "tf-acc-test-where"
  document_id = "where-nomatch"
  fields      = jsonencode({ env = "prod" })
}
data "firestore_documents" "test" {
  collection = "tf-acc-test-where"
  where = [{
    field    = "env"
    operator = "EQUAL"
    value    = "test"
  }]
  depends_on = [firestore_document.match, firestore_document.nomatch]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents"),
						knownvalue.ListSizeExact(1),
					),
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents").AtSliceIndex(0).AtMapKey("fields_map").AtMapKey("env"),
						knownvalue.StringExact("test"),
					),
				},
			},
		},
	})
}

func TestAccDocumentsDataSource_whereMultiple(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "both" {
  collection  = "tf-acc-test-multi-where"
  document_id = "multi-match"
  fields      = jsonencode({ role = "admin", status = "active" })
}
resource "firestore_document" "one" {
  collection  = "tf-acc-test-multi-where"
  document_id = "multi-partial"
  fields      = jsonencode({ role = "admin", status = "inactive" })
}
data "firestore_documents" "test" {
  collection = "tf-acc-test-multi-where"
  where = [
    { field = "role",   operator = "EQUAL", value = "admin" },
    { field = "status", operator = "EQUAL", value = "active" },
  ]
  depends_on = [firestore_document.both, firestore_document.one]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents"),
						knownvalue.ListSizeExact(1),
					),
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents").AtSliceIndex(0).AtMapKey("fields_map").AtMapKey("status"),
						knownvalue.StringExact("active"),
					),
				},
			},
		},
	})
}

func TestAccDocumentsDataSource_limit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "x" {
  collection  = "tf-acc-test-limit"
  document_id = "limit-x"
  fields      = jsonencode({ n = "x" })
}
resource "firestore_document" "y" {
  collection  = "tf-acc-test-limit"
  document_id = "limit-y"
  fields      = jsonencode({ n = "y" })
}
resource "firestore_document" "z" {
  collection  = "tf-acc-test-limit"
  document_id = "limit-z"
  fields      = jsonencode({ n = "z" })
}
data "firestore_documents" "test" {
  collection = "tf-acc-test-limit"
  limit      = 2
  depends_on = [firestore_document.x, firestore_document.y, firestore_document.z]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents"),
						knownvalue.ListSizeExact(2),
					),
				},
			},
		},
	})
}

func TestAccDocumentsDataSource_documentsMap_keys(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "p" {
  collection  = "tf-acc-test-map"
  document_id = "map-doc-p"
  fields      = jsonencode({ label = "p" })
}
data "firestore_documents" "test" {
  collection = "tf-acc-test-map"
  depends_on = [firestore_document.p]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents_map").AtMapKey("map-doc-p").AtMapKey("document_id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue("data.firestore_documents.test",
						tfjsonpath.New("documents_map").AtMapKey("map-doc-p").AtMapKey("fields_map").AtMapKey("label"),
						knownvalue.StringExact("p"),
					),
				},
			},
		},
	})
}

// TestAccDocumentsDataSource_emptyCollection verifies that an empty collection
// produces a plan-time error (failure mode 9: input validation gaps).
func TestAccDocumentsDataSource_emptyCollection(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_documents" "test" {
  collection = ""
}`,
				ExpectError: regexp.MustCompile(`(?i)length must be at least 1`),
			},
		},
	})
}

// TestAccDocumentsDataSource_limitZero verifies that limit=0 produces a
// plan-time error (failure mode 9: input validation gaps).
func TestAccDocumentsDataSource_limitZero(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_documents" "test" {
  collection = "tf-acc-test"
  limit      = 0
}`,
				ExpectError: regexp.MustCompile(`(?i)must be at least 1`),
			},
		},
	})
}

// TestAccDocumentsDataSource_invalidOperator verifies that an invalid operator
// produces a plan-time error (failure mode 4: missing input validation).
func TestAccDocumentsDataSource_invalidOperator(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_documents" "test" {
  collection = "tf-acc-test"
  where = [{
    field    = "status"
    operator = "INVALID_OPERATOR"
    value    = "active"
  }]
}`,
				ExpectError: regexp.MustCompile(`.`),
			},
		},
	})
}

// TestAccDocumentsDataSource_invalidDirection verifies that an invalid direction
// produces a plan-time error (failure mode 4: missing input validation).
func TestAccDocumentsDataSource_invalidDirection(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_documents" "test" {
  collection = "tf-acc-test"
  order_by = [{
    field     = "name"
    direction = "SIDEWAYS"
  }]
}`,
				ExpectError: regexp.MustCompile(`.`),
			},
		},
	})
}

// TestDocumentsDataSourceSchema_mapKeyAttribute verifies the map_key attribute exists
// and is Optional (failure modes 21-25: map_key validation).
func TestDocumentsDataSourceSchema_mapKeyAttribute(t *testing.T) {
	ctx := context.Background()
	ds := NewDocumentsDataSource()
	schemaResp := datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["map_key"]
	if !ok {
		t.Fatal("map_key attribute missing from schema")
	}

	listAttr, ok := attr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("map_key should be ListAttribute, got %T", attr)
	}

	if !listAttr.Optional {
		t.Error("map_key should be Optional")
	}
}

// TestDocumentsDataSourceSchema_mapKeySeparatorAttribute verifies the map_key_separator
// attribute exists and is Optional (failure mode 26: separator collision).
func TestDocumentsDataSourceSchema_mapKeySeparatorAttribute(t *testing.T) {
	ctx := context.Background()
	ds := NewDocumentsDataSource()
	schemaResp := datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["map_key_separator"]
	if !ok {
		t.Fatal("map_key_separator attribute missing from schema")
	}

	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("map_key_separator should be StringAttribute, got %T", attr)
	}

	if !strAttr.Optional {
		t.Error("map_key_separator should be Optional")
	}
}

func firestoreDoc(project, collection, docID, fieldName, fieldValue string) map[string]interface{} {
	return map[string]interface{}{
		"name": fmt.Sprintf("projects/%s/databases/(default)/documents/%s/%s", project, collection, docID),
		"fields": map[string]interface{}{
			fieldName: map[string]interface{}{
				"stringValue": fieldValue,
			},
		},
		"createTime": "2024-01-01T00:00:00Z",
		"updateTime": "2024-01-01T00:00:00Z",
	}
}

// TestUnitListDocuments_pagination verifies that listDocuments follows
// nextPageToken across multiple pages and accumulates all documents.
// Covers failure mode 18: silent truncation of large collections.
func TestUnitListDocuments_pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		token := r.URL.Query().Get("pageToken")

		var resp map[string]interface{}
		switch token {
		case "":
			resp = map[string]interface{}{
				"documents": []interface{}{
					firestoreDoc("test-project", "col", "doc1", "name", "val1"),
					firestoreDoc("test-project", "col", "doc2", "name", "val2"),
				},
				"nextPageToken": "page2",
			}
		case "page2":
			resp = map[string]interface{}{
				"documents": []interface{}{
					firestoreDoc("test-project", "col", "doc3", "name", "val3"),
				},
				"nextPageToken": "page3",
			}
		case "page3":
			resp = map[string]interface{}{
				"documents": []interface{}{
					firestoreDoc("test-project", "col", "doc4", "name", "val4"),
				},
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":{"message":"unexpected pageToken: %s"}}`, token)
			return
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	ds := &DocumentsDataSource{
		client: &FirestoreClient{
			HTTPClient: httpClient,
			Project:    "test-project",
			Database:   "(default)",
		},
	}

	ctx := context.Background()
	docs, diags := ds.listDocuments(ctx, "test-project", "(default)", "col", nil)

	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}
	if len(diags.Warnings()) != 0 {
		t.Fatalf("unexpected warnings: %v", diags.Warnings())
	}
	if len(docs) != 4 {
		t.Fatalf("expected 4 documents, got %d", len(docs))
	}
	if callCount != 3 {
		t.Fatalf("expected 3 HTTP calls (3 pages), got %d", callCount)
	}

	expectedIDs := []string{"doc1", "doc2", "doc3", "doc4"}
	for i, id := range expectedIDs {
		if docs[i].DocumentID.ValueString() != id {
			t.Errorf("document %d: expected ID %q, got %q", i, id, docs[i].DocumentID.ValueString())
		}
	}
}

// TestUnitListDocuments_singlePage verifies that listDocuments returns
// results correctly when no pagination is needed.
func TestUnitListDocuments_singlePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageToken") != "" {
			t.Error("unexpected second page request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"documents": []interface{}{
				firestoreDoc("test-project", "col", "only1", "k", "v1"),
				firestoreDoc("test-project", "col", "only2", "k", "v2"),
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	ds := &DocumentsDataSource{
		client: &FirestoreClient{
			HTTPClient: httpClient,
			Project:    "test-project",
			Database:   "(default)",
		},
	}

	ctx := context.Background()
	docs, diags := ds.listDocuments(ctx, "test-project", "(default)", "col", nil)

	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
}

func makeDocResult(docID string, fields map[string]string) DocumentResult {
	mapVals := make(map[string]attr.Value, len(fields))
	for k, v := range fields {
		mapVals[k] = types.StringValue(v)
	}
	return DocumentResult{
		DocumentID: types.StringValue(docID),
		Fields:     types.StringValue("{}"),
		FieldsMap:  types.MapValueMust(types.StringType, mapVals),
		CreateTime: types.StringValue("2024-01-01T00:00:00Z"),
		UpdateTime: types.StringValue("2024-01-01T00:00:00Z"),
	}
}

// TestResolveDocumentMapKey_singleField verifies single-field map_key resolution
// (failure mode 21: missing map_key field).
func TestResolveDocumentMapKey_singleField(t *testing.T) {
	doc := makeDocResult("doc1", map[string]string{"name": "alice"})
	key, err := resolveDocumentMapKey(doc, []string{"name"}, "_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "alice" {
		t.Errorf("expected key \"alice\", got %q", key)
	}
}

// TestResolveDocumentMapKey_compositeKey verifies multi-field map_key concatenation
// with default separator (failure modes 24-25).
func TestResolveDocumentMapKey_compositeKey(t *testing.T) {
	doc := makeDocResult("doc1", map[string]string{"region": "us-east-1", "env": "prod"})
	key, err := resolveDocumentMapKey(doc, []string{"region", "env"}, "_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "us-east-1_prod" {
		t.Errorf("expected key \"us-east-1_prod\", got %q", key)
	}
}

// TestResolveDocumentMapKey_customSeparator verifies composite key with custom separator.
func TestResolveDocumentMapKey_customSeparator(t *testing.T) {
	doc := makeDocResult("doc1", map[string]string{"region": "us-east-1", "env": "prod"})
	key, err := resolveDocumentMapKey(doc, []string{"region", "env"}, "-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "us-east-1-prod" {
		t.Errorf("expected key \"us-east-1-prod\", got %q", key)
	}
}

// TestResolveDocumentMapKey_missingField verifies error when a composite key field
// is absent from the document (failure mode 24).
func TestResolveDocumentMapKey_missingField(t *testing.T) {
	doc := makeDocResult("doc1", map[string]string{"region": "us-east-1"})
	_, err := resolveDocumentMapKey(doc, []string{"region", "env"}, "_")
	if err == nil {
		t.Fatal("expected error for missing field, got nil")
	}
	if !strings.Contains(err.Error(), "env") {
		t.Errorf("error should mention missing field \"env\", got: %v", err)
	}
}

// TestResolveDocumentMapKey_emptyFieldValue verifies error when a composite key field
// has an empty value (failure mode 25).
func TestResolveDocumentMapKey_emptyFieldValue(t *testing.T) {
	doc := makeDocResult("doc1", map[string]string{"region": "us-east-1", "env": ""})
	_, err := resolveDocumentMapKey(doc, []string{"region", "env"}, "_")
	if err == nil {
		t.Fatal("expected error for empty field value, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty value, got: %v", err)
	}
}

// TestResolveDocumentMapKey_noFields falls back to document_id when mapKeyFields is empty.
func TestResolveDocumentMapKey_noFields(t *testing.T) {
	doc := makeDocResult("doc1", map[string]string{"name": "alice"})
	key, err := resolveDocumentMapKey(doc, nil, "_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "doc1" {
		t.Errorf("expected key \"doc1\", got %q", key)
	}
}

// TestResolveDocumentMapKey_threeFields verifies concatenation with three fields.
func TestResolveDocumentMapKey_threeFields(t *testing.T) {
	doc := makeDocResult("doc1", map[string]string{"a": "x", "b": "y", "c": "z"})
	key, err := resolveDocumentMapKey(doc, []string{"a", "b", "c"}, ":")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "x:y:z" {
		t.Errorf("expected key \"x:y:z\", got %q", key)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
