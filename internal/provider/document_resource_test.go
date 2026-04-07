package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccDocumentResource_autoID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection = "tf-acc-test"
  fields     = jsonencode({ name = "auto-id-test" })
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("firestore_document.test", "document_id"),
					resource.TestCheckResourceAttrSet("firestore_document.test", "name"),
					resource.TestCheckResourceAttrSet("firestore_document.test", "create_time"),
					resource.TestCheckResourceAttrSet("firestore_document.test", "update_time"),
				),
			},
		},
	})
}

func TestAccDocumentResource_explicitID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "explicit-id-test"
  fields      = jsonencode({ name = "explicit" })
}`,
				Check: resource.TestCheckResourceAttr("firestore_document.test", "document_id", "explicit-id-test"),
			},
		},
	})
}

// TestAccDocumentResource_update verifies that update_time does not cause
// perpetual plan drift after an update (failure mode: Section 7 — Diff and Plan Bugs).
func TestAccDocumentResource_update(t *testing.T) {
	configV1 := providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "update-drift-test"
  fields      = jsonencode({ version = "v1" })
}`
	configV2 := providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "update-drift-test"
  fields      = jsonencode({ version = "v2" })
}`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: configV1},
			{
				Config: configV2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccDocumentResource_import_short(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "import-short-test"
  fields      = jsonencode({ name = "import-test" })
}`,
			},
			{
				ResourceName:      "firestore_document.test",
				ImportState:       true,
				ImportStateId:     "tf-acc-test/import-short-test",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccDocumentResource_import_full(t *testing.T) {
	project := projectFromEnv()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "import-full-test"
  fields      = jsonencode({ name = "import-full" })
}`,
			},
			{
				ResourceName:      "firestore_document.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/(default)/tf-acc-test/import-full-test", project),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccDocumentResource_forceNew_collection(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test-a"
  document_id = "forcenew-test"
  fields      = jsonencode({ name = "original" })
}`,
			},
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test-b"
  document_id = "forcenew-test"
  fields      = jsonencode({ name = "original" })
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("firestore_document.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}
