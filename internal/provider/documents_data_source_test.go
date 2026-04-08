package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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

		var resp map[string]interface{}
		token := r.URL.Query().Get("pageToken")

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
			t.Fatalf("unexpected pageToken: %s", token)
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ds := &DocumentsDataSource{
		client: &FirestoreClient{
			HTTPClient: srv.Client(),
			Project:    "test-project",
			Database:   "(default)",
		},
	}

	// Override the base URL by pointing the HTTP client at the test server.
	// listDocuments builds URLs from project/database/collection, so we
	// intercept via a custom transport that rewrites the host.
	origTransport := srv.Client().Transport
	srv.Client().Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		if origTransport != nil {
			return origTransport.RoundTrip(req)
		}
		return http.DefaultTransport.RoundTrip(req)
	})

	ctx := context.Background()
	docs, diags := ds.listDocuments(ctx, "test-project", "(default)", "col", nil)

	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}
	if len(diags.Warnings()) != 0 {
		t.Fatalf("unexpected warnings: %s", diags.Warnings())
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
			t.Fatal("unexpected second page request")
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

	ds := &DocumentsDataSource{
		client: &FirestoreClient{
			HTTPClient: srv.Client(),
			Project:    "test-project",
			Database:   "(default)",
		},
	}

	srv.Client().Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	ctx := context.Background()
	docs, diags := ds.listDocuments(ctx, "test-project", "(default)", "col", nil)

	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags.Errors())
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
