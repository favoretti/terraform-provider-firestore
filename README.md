# Terraform Provider for Google Cloud Firestore

A Terraform provider for managing Google Cloud Firestore documents.

[![release](https://github.com/favoretti/terraform-provider-firestore/actions/workflows/release.yml/badge.svg)](https://github.com/favoretti/terraform-provider-firestore/actions/workflows/release.yml)

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building)
- A Google Cloud project with Firestore enabled

## Installation

### From Source

```bash
git clone https://github.com/favoretti/terraform-provider-firestore
cd terraform-provider-firestore
make install
```

## Authentication

The provider supports two authentication methods:

1. **Application Default Credentials (ADC)** - Recommended for local development
   ```bash
   gcloud auth application-default login
   ```

2. **Service Account JSON** - For production or CI/CD
   ```hcl
   provider "firestore" {
     project     = "my-project"
     credentials = file("path/to/service-account.json")
   }
   ```

### Environment Variables

- `GOOGLE_PROJECT` or `GOOGLE_CLOUD_PROJECT` - Default GCP project
- `GOOGLE_CREDENTIALS` or `GOOGLE_APPLICATION_CREDENTIALS` - Path to credentials file or JSON content

## Usage

### Provider Configuration

```hcl
terraform {
  required_providers {
    firestore = {
      source = "registry.terraform.io/favoretti/firestore"
    }
  }
}

provider "firestore" {
  project  = "my-gcp-project"
  database = "(default)"  # Optional, defaults to "(default)"
}
```

### Managing Documents

```hcl
# Create a document
resource "firestore_document" "user" {
  # project     = "my-gcp-project" # Optional, overrides provider setting
  # database    = "(default)"     # Optional, overrides provider setting
  collection  = "users"
  document_id = "user-123" # Optional, auto-generated if not provided
  fields = jsonencode({
    name  = "John Doe"
    email = "john@example.com"
    age   = 30
  })
}

# Create a subcollection document
resource "firestore_document" "order" {
  collection = "users/user-123/orders"
  fields = jsonencode({
    product = "Widget"
    quantity = 5
  })
}
```

### Reading Documents

```hcl
# Read a single document by ID
data "firestore_document" "user" {
  collection  = "users"
  document_id = "user-123"
}

# Read a single document with field projection
data "firestore_document" "user_name" {
  collection  = "users"
  document_id = "user-123"
  select      = ["name", "email"]
}

# List all documents in a collection
data "firestore_documents" "all_users" {
  collection = "users"
}

# Query with filters, ordering, and field projection
data "firestore_documents" "active_users" {
  collection = "users"
  select     = ["name", "email", "status"]

  where {
    field    = "status"
    operator = "EQUAL"
    value    = "active"
  }

  order_by {
    field     = "created_at"
    direction = "DESCENDING"
  }

  limit = 100
}

# Use map_key for meaningful for_each keys
data "firestore_documents" "apps" {
  collection = "fcp-app-onboarding"
  map_key    = ["name"]
}

# Composite map_key with custom separator
data "firestore_documents" "envs" {
  collection        = "environments"
  map_key           = ["region", "env"]
  map_key_separator = "-"
}

resource "some_resource" "app" {
  for_each = data.firestore_documents.apps.documents_map
  name     = each.key
  eai      = each.value.fields_map["eai"]
}
```

## Resources

### firestore_document

Manages a Firestore document.

#### Arguments

- `collection` (Required) - Collection path (e.g., "users" or "users/123/orders")
- `document_id` (Optional) - Document ID. Auto-generated if not provided.
- `fields` (Required) - JSON string of document fields
- `project` (Optional) - GCP project. Overrides provider setting.
- `database` (Optional) - Firestore database ID. Overrides provider setting.

#### Attributes

- `name` - Full document resource name
- `create_time` - Document creation timestamp
- `update_time` - Document last update timestamp

#### Import

Documents can be imported using the format:

```bash
# Full format
terraform import firestore_document.example project/database/collection/document_id

# Short format (uses provider defaults)
terraform import firestore_document.example collection/document_id
```

## Data Sources

### firestore_document

Retrieves a single Firestore document by ID.

#### Arguments

- `collection` (Required) - Collection path
- `document_id` (Required) - Document ID
- `select` (Optional) - List of field paths to return
- `project` (Optional) - GCP project. Overrides provider setting.
- `database` (Optional) - Firestore database ID. Overrides provider setting.

#### Attributes

- `fields` - JSON string of document fields
- `fields_map` - Top-level fields serialized as strings. Complex values (maps, arrays, geopoints) are JSON-encoded.
- `name` - Full document resource name
- `create_time` - Document creation timestamp
- `update_time` - Document last update timestamp

### firestore_documents

Lists documents in a collection with optional filtering. Automatically paginates large collections.

#### Arguments

- `collection` (Required) - Collection path
- `where` (Optional) - Filter conditions block
  - `field` - Field path to filter on
  - `operator` - Comparison operator (EQUAL, NOT_EQUAL, LESS_THAN, GREATER_THAN, etc.)
  - `value` - JSON-encoded value to compare
- `order_by` (Optional) - Ordering block
  - `field` - Field path to order by
  - `direction` - ASCENDING or DESCENDING
- `limit` (Optional) - Maximum documents to return
- `select` (Optional) - List of field paths to return
- `map_key` (Optional) - List of field names to key `documents_map` by (defaults to `document_id`). When multiple fields are provided, their values are concatenated with `map_key_separator`.
- `map_key_separator` (Optional) - Separator for composite `map_key` values. Defaults to `_`.
- `project` (Optional) - GCP project. Overrides provider setting.
- `database` (Optional) - Firestore database ID. Overrides provider setting.

#### Attributes

- `documents` - List of documents, each containing:
  - `document_id` - Document ID
  - `fields` - JSON string of fields
  - `fields_map` - Top-level fields serialized as strings. Complex values are JSON-encoded.
  - `create_time` - Creation timestamp
  - `update_time` - Update timestamp
- `documents_map` - Documents indexed by `document_id` (or by the fields specified in `map_key`), for use with `for_each`

## Development

```bash
# Build
make build

# Run tests
make test

# Run acceptance tests (requires GCP credentials)
make testacc

# Lint
make lint

# Install locally
make install
```

## License

Mozilla Public License Version 2.0
