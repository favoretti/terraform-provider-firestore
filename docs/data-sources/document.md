---
page_title: "firestore_document Data Source"
description: |-
  Retrieves a single Firestore document by ID.
---

# firestore_document (Data Source)

Retrieves a single Firestore document by its document ID.

## Example Usage

### Basic Lookup

```hcl
data "firestore_document" "user" {
  collection  = "users"
  document_id = "user-123"
}

output "user_name" {
  value = jsondecode(data.firestore_document.user.fields).name
}
```

### Using fields_map

```hcl
data "firestore_document" "app" {
  collection  = "fcp-app-onboarding"
  document_id = "app-123"
}

output "app_eai" {
  value = data.firestore_document.app.fields_map["eai"]
}

output "app_location" {
  value = jsondecode(data.firestore_document.app.fields_map["location"])
}
```

### Field Projection

```hcl
data "firestore_document" "user" {
  collection  = "users"
  document_id = "user-123"
  select      = ["name", "email"]
}
```

### Subcollection Document

```hcl
data "firestore_document" "order" {
  collection  = "users/user-123/orders"
  document_id = "order-001"
}
```

## Schema

### Required

- `collection` (String) - The collection path (e.g., `"users"` or `"users/123/orders"`).
- `document_id` (String) - The document ID to retrieve.

### Optional

- `project` (String) - The GCP project ID. Overrides the provider project.
- `database` (String) - The Firestore database ID. Overrides the provider database.
- `select` (List of String) - List of field paths to return. If omitted, all fields are returned. Must contain at least one entry.

### Read-Only

- `fields` (String) - JSON string of all document fields.
- `fields_map` (Map of String) - Top-level fields serialized as strings. Complex values (maps, arrays, geopoints) are JSON-encoded.
- `name` (String) - The full document resource name.
- `create_time` (String) - The time the document was created.
- `update_time` (String) - The time the document was last updated.
