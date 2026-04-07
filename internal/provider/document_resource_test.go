package provider

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"golang.org/x/oauth2/google"
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
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("document_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("name"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("create_time"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("update_time"), knownvalue.NotNull()),
				},
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
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("document_id"), knownvalue.StringExact("explicit-id-test")),
				},
			},
		},
	})
}

// TestAccDocumentResource_update verifies that update_time does not cause
// perpetual plan drift after an update (failure mode 1: state drift).
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

// TestAccDocumentResource_disappears verifies that the provider detects external deletion
// and plans to recreate the resource (failure mode 8: state removal on transient errors).
func TestAccDocumentResource_disappears(t *testing.T) {
	project := projectFromEnv()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "disappears-test"
  fields      = jsonencode({ name = "disappears" })
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("document_id"), knownvalue.StringExact("disappears-test")),
				},
			},
			{
				PreConfig: func() {
					testAccDeleteFirestoreDocument(t, project, "(default)", "tf-acc-test", "disappears-test")
				},
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "disappears-test"
  fields      = jsonencode({ name = "disappears" })
}`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccDocumentResource_complexFields verifies round-trip fidelity for arrays,
// nested maps, integers, and booleans (failure mode 3: silent type corruption).
func TestAccDocumentResource_complexFields(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "complex-fields-test"
  fields = jsonencode({
    str    = "hello"
    count  = 42
    flag   = true
    arr    = ["a", "b", "c"]
    nested = { key = "val", n = 1 }
  })
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("document_id"), knownvalue.StringExact("complex-fields-test")),
					statecheck.ExpectKnownValue("firestore_document.test", tfjsonpath.New("fields"), knownvalue.NotNull()),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "firestore_document.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// testAccDeleteFirestoreDocument deletes a Firestore document directly via API,
// bypassing Terraform. Used to simulate external deletion in disappears tests.
func testAccDeleteFirestoreDocument(t *testing.T, project, database, collection, docID string) {
	t.Helper()
	ctx := context.Background()

	creds, err := google.FindDefaultCredentials(ctx,
		"https://www.googleapis.com/auth/datastore",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		t.Fatalf("finding credentials for document cleanup: %v", err)
	}

	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s/%s",
		project, database, collection, docID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		t.Fatalf("building delete request: %v", err)
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		t.Fatalf("getting token for document cleanup: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("deleting document %s/%s: %v", collection, docID, err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected status %d deleting document %s/%s", resp.StatusCode, collection, docID)
	}
}
