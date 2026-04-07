package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

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
