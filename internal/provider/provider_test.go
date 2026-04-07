package provider

import (
	"context"
	"os"
	"testing"

	providerfw "github.com/hashicorp/terraform-plugin-framework/provider"
	providerSchema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
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
