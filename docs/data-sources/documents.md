---
page_title: "firestore_documents Data Source"
description: |-
  Lists Firestore documents in a collection.
---

# firestore_documents (Data Source)

Lists documents in a Firestore collection with optional filtering, ordering, and field projection. Automatically paginates large collections.

## Example Usage

### List All Documents

```hcl
data "firestore_documents" "all_users" {
  collection = "users"
}

output "user_count" {
  value = length(data.firestore_documents.all_users.documents)
}
```

### Filter Documents

```hcl
data "firestore_documents" "active_users" {
  collection = "users"
  where {
    field    = "active"
    operator = "EQUAL"
    value    = jsonencode(true)
  }
}
```

### Multiple Filters (AND)

```hcl
data "firestore_documents" "premium_active_users" {
  collection = "users"
  where {
    field    = "active"
    operator = "EQUAL"
    value    = jsonencode(true)
  }
  where {
    field    = "tier"
    operator = "EQUAL"
    value    = "premium"
  }
}
```

### Order and Limit

```hcl
data "firestore_documents" "recent_orders" {
  collection = "orders"
  order_by {
    field     = "created_at"
    direction = "DESCENDING"
  }
  limit = 10
}
```

### Field Projection

```hcl
data "firestore_documents" "network_names" {
  collection = "network"
  select     = ["name", "document_type", "environment"]
}
```

### Custom Map Key

```hcl
data "firestore_documents" "apps" {
  collection = "fcp-app-onboarding"
  map_key    = ["name"]
}

resource "some_resource" "app" {
  for_each = data.firestore_documents.apps.documents_map
  name     = each.key                            # e.g., "fcp-app-example"
  eai      = each.value.fields_map["eai"]        # e.g., "9998002"
  email    = each.value.fields_map["group_email"]
}
```

### Composite Map Key

```hcl
data "firestore_documents" "envs" {
  collection        = "environments"
  map_key           = ["region", "env"]
  map_key_separator = "-"
}

resource "some_resource" "env" {
  for_each = data.firestore_documents.envs.documents_map
  name     = each.key   # e.g., "us-east-1-prod"
}
```

### Using fields_map with Complex Types

```hcl
data "firestore_documents" "networks" {
  collection = "network"
}

output "first_location" {
  value = jsondecode(data.firestore_documents.networks.documents[0].fields_map["location"])
}
```

## Schema

### Required

- `collection` (String) - The collection path (e.g., `"users"` or `"users/123/orders"`).

### Optional

- `project` (String) - The GCP project ID. Overrides the provider project.
- `database` (String) - The Firestore database ID. Overrides the provider database.
- `limit` (Number) - Maximum number of documents to return. Must be at least 1.
- `select` (List of String) - List of field paths to return. If omitted, all fields are returned. Must contain at least one entry.
- `map_key` (List of String) - List of field names to use as the key for `documents_map`. Defaults to `document_id`. When multiple fields are provided, their values are concatenated with `map_key_separator`. Each specified field must exist and have a unique, non-empty value in every returned document. Must contain at least one entry.
- `map_key_separator` (String) - Separator for composite `map_key` values. Defaults to `_`. Choose a separator that does not appear in the field values to avoid key collisions.
- `where` (Block List) - Filter conditions. Multiple blocks are combined with AND. Each block contains:
  - `field` (String, Required) - The field path to filter on.
  - `operator` (String, Required) - The comparison operator. Valid values: `EQUAL`, `NOT_EQUAL`, `LESS_THAN`, `LESS_THAN_OR_EQUAL`, `GREATER_THAN`, `GREATER_THAN_OR_EQUAL`, `ARRAY_CONTAINS`, `IN`, `ARRAY_CONTAINS_ANY`, `NOT_IN`.
  - `value` (String, Required) - The value to compare against. Plain strings can be passed as-is. Use `jsonencode()` for booleans, numbers, arrays, or objects.
- `order_by` (Block List) - Ordering for query results. Each block contains:
  - `field` (String, Required) - The field path to order by.
  - `direction` (String, Optional) - The sort direction. Valid values: `ASCENDING` (default), `DESCENDING`.

### Read-Only

- `documents` (List of Object) - List of documents in the collection. Each entry contains:
  - `document_id` (String) - The document ID.
  - `fields` (String) - JSON string of all document fields.
  - `fields_map` (Map of String) - Top-level fields serialized as strings. Complex values (maps, arrays, geopoints) are JSON-encoded.
  - `create_time` (String) - The time the document was created.
  - `update_time` (String) - The time the document was last updated.
- `documents_map` (Map of Object) - Documents indexed by `document_id` (or by the fields specified in `map_key`), for use with `for_each`. Each entry has the same shape as a `documents` list entry.

## Pagination

The data source automatically paginates through large collections. Collections with more than 300 documents are fetched across multiple API calls. A safety cap of 100 pages (30,000 documents) is enforced; if exceeded, a warning diagnostic is emitted and partial results are returned. Use `where` filters or `limit` to reduce the result set for very large collections.
