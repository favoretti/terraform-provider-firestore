terraform {
  required_providers {
    firestore = {
      source = "favoretti/firestore"
    }
  }
}

# Configure the provider with explicit credentials
provider "firestore" {
  project = "my-gcp-project"
  # credentials = file("path/to/service-account.json")
  # database = "(default)"
}

# Or use Application Default Credentials (ADC)
# provider "firestore" {
#   project = "my-gcp-project"
# }
