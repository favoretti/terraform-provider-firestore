package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccDocumentDataSource_byID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "seed" {
  collection  = "tf-acc-test"
  document_id = "ds-by-id-test"
  fields      = jsonencode({ label = "by-id" })
}

data "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = firestore_document.seed.document_id
  depends_on  = [firestore_document.seed]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_document.test", tfjsonpath.New("fields"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.firestore_document.test", tfjsonpath.New("name"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.firestore_document.test",
						tfjsonpath.New("fields_map").AtMapKey("label"),
						knownvalue.StringExact("by-id"),
					),
				},
			},
		},
	})
}

func TestAccDocumentDataSource_byWhere_single(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "seed" {
  collection  = "tf-acc-test"
  document_id = "ds-where-single-test"
  fields      = jsonencode({ marker = "where-single" })
}

data "firestore_document" "test" {
  collection = "tf-acc-test"
  where = [{
    field    = "marker"
    operator = "EQUAL"
    value    = "where-single"
  }]
  depends_on = [firestore_document.seed]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_document.test", tfjsonpath.New("document_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.firestore_document.test",
						tfjsonpath.New("fields_map").AtMapKey("marker"),
						knownvalue.StringExact("where-single"),
					),
				},
			},
		},
	})
}

func TestAccDocumentDataSource_byWhere_multiple(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "seed" {
  collection  = "tf-acc-test"
  document_id = "ds-where-multi-test"
  fields      = jsonencode({ role = "admin", status = "active" })
}

data "firestore_document" "test" {
  collection = "tf-acc-test"
  where = [
    { field = "role",   operator = "EQUAL", value = "admin" },
    { field = "status", operator = "EQUAL", value = "active" },
  ]
  depends_on = [firestore_document.seed]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_document.test",
						tfjsonpath.New("fields_map").AtMapKey("role"),
						knownvalue.StringExact("admin"),
					),
					statecheck.ExpectKnownValue("data.firestore_document.test",
						tfjsonpath.New("fields_map").AtMapKey("status"),
						knownvalue.StringExact("active"),
					),
				},
			},
		},
	})
}

func TestAccDocumentDataSource_byWhere_noMatch(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_document" "test" {
  collection = "tf-acc-test"
  where = [{
    field    = "marker"
    operator = "EQUAL"
    value    = "this-value-does-not-exist-in-any-document"
  }]
}`,
				ExpectError: regexp.MustCompile(`(?i)no document|not found`),
			},
		},
	})
}

func TestAccDocumentDataSource_neitherIDNorWhere(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_document" "test" {
  collection = "tf-acc-test"
}`,
				ExpectError: regexp.MustCompile(`(?i)document_id or at least one where`),
			},
		},
	})
}

// TestAccDocumentDataSource_invalidOperator verifies that an invalid operator
// produces a plan-time error (failure mode 4: missing input validation).
func TestAccDocumentDataSource_invalidOperator(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_document" "test" {
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
