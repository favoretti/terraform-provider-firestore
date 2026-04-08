package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccDocumentDataSource_emptyCollection verifies that an empty collection
// produces a plan-time error (failure mode 9: input validation gaps).
func TestAccDocumentDataSource_emptyCollection(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
data "firestore_document" "test" {
  collection  = ""
  document_id = "test"
}`,
				ExpectError: regexp.MustCompile(`(?i)length must be at least 1`),
			},
		},
	})
}

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
