# Terraform Provider for Google Cloud Firestore

A Terraform provider for managing Google Cloud Firestore documents.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building)
- A Google Cloud project with Firestore enabled

## Installation

### From Source

```bash
git clone https://github.com/terraform-provider-firestore
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
# Read a single document
data "firestore_document" "user" {
  collection  = "users"
  document_id = "user-123"
}

# List all documents in a collection
data "firestore_documents" "all_users" {
  collection = "users"
}

# Query with filters
data "firestore_documents" "active_users" {
  collection = "users"

  where {
    field    = "status"
    operator = "EQUAL"
    value    = jsonencode("active")
  }

  order_by {
    field     = "created_at"
    direction = "DESCENDING"
  }

  limit = 100
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

Retrieves a single Firestore document.

#### Arguments

- `collection` (Required) - Collection path
- `document_id` (Required) - Document ID

#### Attributes

- `fields` - JSON string of document fields
- `name` - Full document resource name
- `create_time` - Document creation timestamp
- `update_time` - Document last update timestamp

### firestore_documents

Lists documents in a collection with optional filtering.

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

#### Attributes

- `documents` - List of documents, each containing:
  - `document_id` - Document ID
  - `fields` - JSON string of fields
  - `create_time` - Creation timestamp
  - `update_time` - Update timestamp

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
