package provider

import (
	"context"
	"os"
	"testing"

	providerfw "github.com/hashicorp/terraform-plugin-framework/provider"
	providerSchema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"firestore": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("GOOGLE_PROJECT") == "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Fatal("GOOGLE_PROJECT or GOOGLE_CLOUD_PROJECT must be set for acceptance tests")
	}
	if os.Getenv("GOOGLE_CREDENTIALS") == "" && os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Fatal("GOOGLE_CREDENTIALS or GOOGLE_APPLICATION_CREDENTIALS must be set for acceptance tests")
	}
}

func providerConfig() string {
	return `provider "firestore" {}`
}

func projectFromEnv() string {
	if v := os.Getenv("GOOGLE_PROJECT"); v != "" {
		return v
	}
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}

func nullModel() FirestoreProviderModel {
	return FirestoreProviderModel{
		Project:                   types.StringNull(),
		Database:                  types.StringNull(),
		Credentials:               types.StringNull(),
		ImpersonateServiceAccount: types.StringNull(),
	}
}

func TestResolveProviderConfig_projectFromGOOGLE_PROJECT(t *testing.T) {
	t.Setenv("GOOGLE_PROJECT", "env-project")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	got := resolveProviderConfig(nullModel())
	if got.project != "env-project" {
		t.Errorf("expected env-project, got %q", got.project)
	}
}

func TestResolveProviderConfig_projectFromGOOGLE_CLOUD_PROJECT(t *testing.T) {
	t.Setenv("GOOGLE_PROJECT", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "cloud-project")
	got := resolveProviderConfig(nullModel())
	if got.project != "cloud-project" {
		t.Errorf("expected cloud-project, got %q", got.project)
	}
}

func TestResolveProviderConfig_HCLProjectOverridesEnv(t *testing.T) {
	t.Setenv("GOOGLE_PROJECT", "env-project")
	m := nullModel()
	m.Project = types.StringValue("hcl-project")
	got := resolveProviderConfig(m)
	if got.project != "hcl-project" {
		t.Errorf("expected hcl-project, got %q", got.project)
	}
}

func TestResolveProviderConfig_databaseDefault(t *testing.T) {
	got := resolveProviderConfig(nullModel())
	if got.database != "(default)" {
		t.Errorf("expected (default), got %q", got.database)
	}
}

func TestResolveProviderConfig_credentialsFromGOOGLE_CREDENTIALS(t *testing.T) {
	t.Setenv("GOOGLE_CREDENTIALS", `{"type":"service_account"}`)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	got := resolveProviderConfig(nullModel())
	if got.credentials != `{"type":"service_account"}` {
		t.Errorf("unexpected credentials: %q", got.credentials)
	}
}

func TestResolveProviderConfig_HCLCredentialsOverridesEnv(t *testing.T) {
	t.Setenv("GOOGLE_CREDENTIALS", `{"type":"service_account","from":"env"}`)
	m := nullModel()
	m.Credentials = types.StringValue(`{"type":"service_account","from":"hcl"}`)
	got := resolveProviderConfig(m)
	if got.credentials != `{"type":"service_account","from":"hcl"}` {
		t.Errorf("unexpected credentials: %q", got.credentials)
	}
}

func TestProviderSchema_credentialsSensitive(t *testing.T) {
	ctx := context.Background()
	p := &FirestoreProvider{version: "test"}

	var resp providerfw.SchemaResponse
	p.Schema(ctx, providerfw.SchemaRequest{}, &resp)

	attr, ok := resp.Schema.Attributes["credentials"]
	if !ok {
		t.Fatal("credentials attribute not found in provider schema")
	}
	sa, ok := attr.(providerSchema.StringAttribute)
	if !ok {
		t.Fatalf("credentials is not a StringAttribute, got %T", attr)
	}
	if !sa.Sensitive {
		t.Error("credentials attribute must be marked Sensitive: true")
	}
}
