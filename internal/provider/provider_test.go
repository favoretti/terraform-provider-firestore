package provider

import (
	"os"
	"testing"

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
