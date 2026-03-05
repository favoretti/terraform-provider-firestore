---
page_title: "Firestore Provider"
description: |-
  The Firestore provider is used to manage Google Cloud Firestore documents.
---

# Firestore Provider

The Firestore provider allows you to manage [Google Cloud Firestore](https://cloud.google.com/firestore) documents using Terraform.

## Example Usage

```hcl
terraform {
  required_providers {
    firestore = {
      source = "registry.terraform.io/favoretti/firestore"
    }
  }
}

provider "firestore" {
  project = "my-gcp-project"
}

resource "firestore_document" "example" {
  collection  = "users"
  document_id = "user-123"
  fields = jsonencode({
    name  = "John Doe"
    email = "john@example.com"
  })
}
```

## Authentication

The provider supports two authentication methods:

### Application Default Credentials (ADC)

The recommended approach for local development:

```bash
gcloud auth application-default login
```

### Service Account Credentials

For production or CI/CD environments:

```hcl
provider "firestore" {
  project     = "my-project"
  credentials = file("path/to/service-account.json")
}
```

Or via environment variable:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
```

### Service Account Impersonation

To act as a different service account without managing its key directly, use impersonation. The caller must have the `roles/iam.serviceAccountTokenCreator` role on the target service account.

```hcl
provider "firestore" {
  project                    = "my-project"
  impersonate_service_account = "target@my-project.iam.gserviceaccount.com"
}
```

Or via environment variable:

```bash
export GOOGLE_IMPERSONATE_SERVICE_ACCOUNT="target@my-project.iam.gserviceaccount.com"
```

## Schema

### Optional

- `project` (String) - The GCP project ID. Can also be set via `GOOGLE_PROJECT` or `GOOGLE_CLOUD_PROJECT` environment variables.
- `credentials` (String, Sensitive) - Service account JSON credentials. Can be a file path or JSON string. Can also be set via `GOOGLE_CREDENTIALS` or `GOOGLE_APPLICATION_CREDENTIALS` environment variables.
- `database` (String) - The Firestore database ID. Defaults to `(default)`.
- `impersonate_service_account` (String) - The service account email to impersonate for all API calls. The caller must have `roles/iam.serviceAccountTokenCreator` on the target. Can also be set via the `GOOGLE_IMPERSONATE_SERVICE_ACCOUNT` environment variable.
