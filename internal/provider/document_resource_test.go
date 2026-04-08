package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	fwschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"golang.org/x/oauth2/google"
)

func TestAccDocumentResource_autoID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
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
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
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
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{Config: configV1},
			{
				Config: configV2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("firestore_document.test", plancheck.ResourceActionUpdate),
					},
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
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
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
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
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
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
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
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
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
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
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

func TestAccDocumentResource_forceNew_documentID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "forcenew-docid-v1"
  fields      = jsonencode({ name = "v1" })
}`,
			},
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "forcenew-docid-v2"
  fields      = jsonencode({ name = "v1" })
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

func TestAccDocumentResource_forceNew_database(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
provider "firestore" { database = "(default)" }
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "forcenew-db-test"
  fields      = jsonencode({ name = "db-test" })
}`,
			},
			{
				Config: `
provider "firestore" { database = "(default)" }
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "forcenew-db-test"
  database    = "(default)"
  fields      = jsonencode({ name = "db-test" })
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

func TestAccDocumentResource_databaseDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "db-default-test"
  fields      = jsonencode({ name = "default-db" })
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("firestore_document.test",
						tfjsonpath.New("database"),
						knownvalue.StringExact("(default)"),
					),
				},
			},
		},
	})
}

// TestDocumentResourceSchema_projectRequiresReplace verifies the project attribute
// carries RequiresReplace (failure mode 1: schema stability).
func TestDocumentResourceSchema_projectRequiresReplace(t *testing.T) {
	ctx := context.Background()
	r := &DocumentResource{}

	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["project"]
	if !ok {
		t.Fatal("project attribute not found in document resource schema")
	}
	sa, ok := attr.(fwschema.StringAttribute)
	if !ok {
		t.Fatalf("project is not a StringAttribute, got %T", attr)
	}
	hasRequiresReplace := false
	for _, pm := range sa.PlanModifiers {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%T", pm)), "requiresreplace") {
			hasRequiresReplace = true
		}
	}
	if !hasRequiresReplace {
		t.Error("project attribute must have RequiresReplace plan modifier")
	}
}

// TestAccDocumentResource_invalidFieldsJSON verifies that a non-JSON-object value in
// fields produces a plan-time error (failure mode 4: missing input validation).
func TestAccDocumentResource_invalidFieldsJSON(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection = "tf-acc-test"
  fields     = "not valid json"
}`,
				ExpectError: regexp.MustCompile(`(?i)invalid json|valid json`),
			},
		},
	})
}

// TestAccDocumentResource_updateMaskPreservesUnmanagedField verifies that Update()
// leaves fields not present in the Terraform config untouched in Firestore
// (failure mode 2: data loss on update).
//
// The resource will show a non-empty plan after the apply because Read() stores all
// Firestore fields in state (including the extra field), while the config only
// declares the managed field. This is expected: updateMask prevents deletion but
// does not eliminate the config/state diff caused by unmanaged fields.
func TestAccDocumentResource_updateMaskPreservesUnmanagedField(t *testing.T) {
	project := projectFromEnv()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFirestoreDocumentDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "updatemask-test"
  fields      = jsonencode({ managed = "yes" })
}`,
			},
			{
				PreConfig: func() {
					testAccPatchFirestoreField(t, project, "(default)", "tf-acc-test", "updatemask-test", "extra", "preserved")
				},
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection  = "tf-acc-test"
  document_id = "updatemask-test"
  fields      = jsonencode({ managed = "yes" })
}
data "firestore_document" "verify" {
  collection  = "tf-acc-test"
  document_id = "updatemask-test"
  depends_on  = [firestore_document.test]
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.firestore_document.verify",
						tfjsonpath.New("fields_map").AtMapKey("extra"),
						knownvalue.StringExact("preserved"),
					),
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// testAccCheckFirestoreDocumentDestroy verifies that all firestore_document resources
// in the Terraform state have been deleted from Firestore after destroy
// (failure mode 8: state removal on transient errors).
func testAccCheckFirestoreDocumentDestroy(s *terraform.State) error {
	ctx := context.Background()

	creds, err := google.FindDefaultCredentials(ctx,
		"https://www.googleapis.com/auth/datastore",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return fmt.Errorf("finding credentials for destroy check: %w", err)
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("getting token for destroy check: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "firestore_document" {
			continue
		}

		project := rs.Primary.Attributes["project"]
		database := rs.Primary.Attributes["database"]
		collection := rs.Primary.Attributes["collection"]
		documentID := rs.Primary.Attributes["document_id"]

		reqURL := fmt.Sprintf(
			"https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s/%s",
			project, database, collection, documentID,
		)

		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return fmt.Errorf("building destroy-check request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing destroy-check request for %s/%s: %w", collection, documentID, err)
		}
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusNotFound:
			continue
		case http.StatusOK:
			return fmt.Errorf("firestore document %s/%s still exists after destroy", collection, documentID)
		default:
			return fmt.Errorf("unexpected status %d checking destruction of %s/%s", resp.StatusCode, collection, documentID)
		}
	}

	return nil
}

// testAccPatchFirestoreField adds a single string field to an existing Firestore
// document via the REST API using updateMask, leaving all other fields unchanged.
func testAccPatchFirestoreField(t *testing.T, project, database, collection, docID, field, value string) {
	t.Helper()
	ctx := context.Background()

	creds, err := google.FindDefaultCredentials(ctx,
		"https://www.googleapis.com/auth/datastore",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		t.Fatalf("finding credentials for field patch: %v", err)
	}

	reqURL := fmt.Sprintf(
		"https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s/%s?updateMask.fieldPaths=%s",
		project, database, collection, docID, field,
	)

	body := fmt.Sprintf(`{"fields":{%q:{"stringValue":%q}}}`, field, value)
	req, err := http.NewRequestWithContext(ctx, "PATCH", reqURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building patch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	token, err := creds.TokenSource.Token()
	if err != nil {
		t.Fatalf("getting token for field patch: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patching field %s/%s.%s: %v", collection, docID, field, err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d patching field %s/%s.%s", resp.StatusCode, collection, docID, field)
	}
}

// TestDocumentResourceSchema_collectionRequiresReplace verifies the collection attribute
// carries RequiresReplace (failure mode 1: schema stability).
func TestDocumentResourceSchema_collectionRequiresReplace(t *testing.T) {
	ctx := context.Background()
	r := &DocumentResource{}

	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["collection"]
	if !ok {
		t.Fatal("collection attribute not found in document resource schema")
	}
	sa, ok := attr.(fwschema.StringAttribute)
	if !ok {
		t.Fatalf("collection is not a StringAttribute, got %T", attr)
	}
	hasRequiresReplace := false
	for _, pm := range sa.PlanModifiers {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%T", pm)), "requiresreplace") {
			hasRequiresReplace = true
		}
	}
	if !hasRequiresReplace {
		t.Error("collection attribute must have RequiresReplace plan modifier")
	}
}

// TestDocumentResourceSchema_documentIDRequiresReplace verifies the document_id attribute
// carries RequiresReplace (failure mode 1: schema stability).
func TestDocumentResourceSchema_documentIDRequiresReplace(t *testing.T) {
	ctx := context.Background()
	r := &DocumentResource{}

	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["document_id"]
	if !ok {
		t.Fatal("document_id attribute not found in document resource schema")
	}
	sa, ok := attr.(fwschema.StringAttribute)
	if !ok {
		t.Fatalf("document_id is not a StringAttribute, got %T", attr)
	}
	hasRequiresReplace := false
	for _, pm := range sa.PlanModifiers {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%T", pm)), "requiresreplace") {
			hasRequiresReplace = true
		}
	}
	if !hasRequiresReplace {
		t.Error("document_id attribute must have RequiresReplace plan modifier")
	}
}

// TestDocumentResourceSchema_databaseRequiresReplace verifies the database attribute
// carries RequiresReplace (failure mode 1: schema stability).
func TestDocumentResourceSchema_databaseRequiresReplace(t *testing.T) {
	ctx := context.Background()
	r := &DocumentResource{}

	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["database"]
	if !ok {
		t.Fatal("database attribute not found in document resource schema")
	}
	sa, ok := attr.(fwschema.StringAttribute)
	if !ok {
		t.Fatalf("database is not a StringAttribute, got %T", attr)
	}
	hasRequiresReplace := false
	for _, pm := range sa.PlanModifiers {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%T", pm)), "requiresreplace") {
			hasRequiresReplace = true
		}
	}
	if !hasRequiresReplace {
		t.Error("database attribute must have RequiresReplace plan modifier")
	}
}

// TestDocumentResourceSchema_fieldsNoRequiresReplace verifies that the fields attribute
// does NOT carry RequiresReplace, confirming in-place updates (failure mode 5: outage patterns).
func TestDocumentResourceSchema_fieldsNoRequiresReplace(t *testing.T) {
	ctx := context.Background()
	r := &DocumentResource{}

	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["fields"]
	if !ok {
		t.Fatal("fields attribute not found in document resource schema")
	}
	sa, ok := attr.(fwschema.StringAttribute)
	if !ok {
		t.Fatalf("fields is not a StringAttribute, got %T", attr)
	}
	for _, pm := range sa.PlanModifiers {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%T", pm)), "requiresreplace") {
			t.Error("fields attribute must NOT have RequiresReplace plan modifier")
		}
	}
}

// TestDocumentResourceSchema_computedAttributesUseStateForUnknown verifies all computed
// attributes use UseStateForUnknown to prevent plan drift (failure mode 1, 7).
func TestDocumentResourceSchema_computedAttributesUseStateForUnknown(t *testing.T) {
	ctx := context.Background()
	r := &DocumentResource{}

	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	computedAttrs := []string{"name", "create_time", "update_time", "document_id", "project", "database"}

	for _, attrName := range computedAttrs {
		t.Run(attrName, func(t *testing.T) {
			attr, ok := schemaResp.Schema.Attributes[attrName]
			if !ok {
				t.Fatalf("%s attribute not found in document resource schema", attrName)
			}
			sa, ok := attr.(fwschema.StringAttribute)
			if !ok {
				t.Fatalf("%s is not a StringAttribute, got %T", attrName, attr)
			}
			hasUseStateForUnknown := false
			for _, pm := range sa.PlanModifiers {
				if strings.Contains(strings.ToLower(fmt.Sprintf("%T", pm)), "usestateforunknown") {
					hasUseStateForUnknown = true
				}
			}
			if !hasUseStateForUnknown {
				t.Errorf("%s attribute must have UseStateForUnknown plan modifier", attrName)
			}
		})
	}
}

// TestAccDocumentResource_emptyCollection verifies that an empty collection
// produces a plan-time error (failure mode 9: input validation gaps).
func TestAccDocumentResource_emptyCollection(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "firestore_document" "test" {
  collection = ""
  fields     = jsonencode({ name = "test" })
}`,
				ExpectError: regexp.MustCompile(`(?i)length must be at least 1`),
			},
		},
	})
}
