---
page_title: "firestore_document Data Source"
description: |-
  Retrieves a single Firestore document by ID or by filter conditions.
---

# firestore_document (Data Source)

Retrieves a single Firestore document. Use `document_id` for a direct lookup, or `where` to find the first document matching one or more field conditions.

## Example Usage

### Lookup by Document ID

```hcl
data "firestore_document" "user" {
  collection  = "users"
  document_id = "user-123"
}

output "user_name" {
  value = jsondecode(data.firestore_document.user.fields).name
}
```

### Lookup by Field Value

```hcl
data "firestore_document" "user" {
  collection = "users"
  where = [{
    field    = "email"
    operator = "EQUAL"
    value    = "alice@example.com"
  }]
}

output "user_name" {
  value = data.firestore_document.user.fields_map["display_name"]
}
```

### Multiple Filter Conditions (AND)

```hcl
data "firestore_document" "active_admin" {
  collection = "users"
  where = [
    { field = "role",   operator = "EQUAL", value = "admin" },
    { field = "active", operator = "EQUAL", value = jsonencode(true) },
  ]
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

### Optional

- `project` (String) - The GCP project ID. Overrides the provider project.
- `database` (String) - The Firestore database ID. Overrides the provider database.
- `document_id` (String) - The document ID to retrieve. Mutually exclusive with `where`.
- `where` (List of Object) - Filter conditions. The first matching document is returned. Multiple entries are combined with AND. Requires at least one entry when `document_id` is not set. Each entry contains:
  - `field` (String, Required) - The field path to filter on.
  - `operator` (String, Required) - The comparison operator. Valid values: `EQUAL`, `NOT_EQUAL`, `LESS_THAN`, `LESS_THAN_OR_EQUAL`, `GREATER_THAN`, `GREATER_THAN_OR_EQUAL`, `ARRAY_CONTAINS`, `IN`, `ARRAY_CONTAINS_ANY`, `NOT_IN`.
  - `value` (String, Required) - The value to compare against. Plain strings can be passed as-is. Use `jsonencode()` for booleans, numbers, arrays, or objects.

### Read-Only

- `document_id` (String) - The document ID. Populated from the matched document when using `where`.
- `fields` (String) - JSON string of all document fields.
- `fields_map` (Map of String) - Top-level string-valued fields as a map. Non-string and nested fields are omitted.
- `name` (String) - The full document resource name.
- `create_time` (String) - The time the document was created.
- `update_time` (String) - The time the document was last updated.
