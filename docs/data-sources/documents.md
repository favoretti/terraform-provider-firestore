---
page_title: "firestore_documents Data Source"
description: |-
  Lists Firestore documents in a collection.
---

# firestore_documents (Data Source)

Lists documents in a Firestore collection with optional filtering, ordering, and pagination.

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

### Multiple Filters

Multiple top-level `where` blocks are combined using the `filter_operator` attribute (defaults to `AND`).

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
    value    = jsonencode("premium")
  }
}
```

### OR Filters

Use `filter_operator = "OR"` to combine top-level `where` blocks with OR logic.

```hcl
data "firestore_documents" "admins_or_editors" {
  collection      = "users"
  filter_operator = "OR"

  where {
    field    = "role"
    operator = "EQUAL"
    value    = jsonencode("admin")
  }

  where {
    field    = "role"
    operator = "EQUAL"
    value    = jsonencode("editor")
  }
}
```

### Nested Filters with where_group

Use `where_group` blocks to nest AND/OR logic. Each group has its own `group_operator` and is combined with other top-level conditions via `filter_operator`.

```hcl
# status = "active" AND (role = "admin" OR role = "editor")
data "firestore_documents" "active_privileged_users" {
  collection      = "users"
  filter_operator = "AND"

  where {
    field    = "status"
    operator = "EQUAL"
    value    = jsonencode("active")
  }

  where_group {
    group_operator = "OR"

    where {
      field    = "role"
      operator = "EQUAL"
      value    = jsonencode("admin")
    }

    where {
      field    = "role"
      operator = "EQUAL"
      value    = jsonencode("editor")
    }
  }
}
```

### Multiple Groups

Multiple `where_group` blocks can be combined for more complex logic.

```hcl
# (role = "admin" OR role = "editor") AND (dept = "engineering" OR dept = "product")
data "firestore_documents" "privileged_in_depts" {
  collection      = "users"
  filter_operator = "AND"

  where_group {
    group_operator = "OR"

    where {
      field    = "role"
      operator = "EQUAL"
      value    = jsonencode("admin")
    }

    where {
      field    = "role"
      operator = "EQUAL"
      value    = jsonencode("editor")
    }
  }

  where_group {
    group_operator = "OR"

    where {
      field    = "dept"
      operator = "EQUAL"
      value    = jsonencode("engineering")
    }

    where {
      field    = "dept"
      operator = "EQUAL"
      value    = jsonencode("product")
    }
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

### Complex Query

```hcl
data "firestore_documents" "filtered_users" {
  collection = "users"

  where {
    field    = "age"
    operator = "GREATER_THAN_OR_EQUAL"
    value    = jsonencode(18)
  }

  order_by {
    field     = "age"
    direction = "ASCENDING"
  }

  order_by {
    field     = "name"
    direction = "ASCENDING"
  }

  limit = 100
}
```

## Schema

### Required

- `collection` (String) - The collection path (e.g., "users" or "users/123/orders").

### Optional

- `project` (String) - The GCP project ID. Overrides the provider project.
- `database` (String) - The Firestore database ID. Overrides the provider database.
- `filter_operator` (String) - The operator used to combine top-level `where` blocks and `where_group` blocks. Valid values: `AND` (default), `OR`.
- `limit` (Number) - Maximum number of documents to return.

### Blocks

#### where (Optional)

Filter conditions for the query. Multiple `where` blocks are combined using `filter_operator`.

- `field` (String, Required) - The field path to filter on.
- `operator` (String, Required) - The comparison operator. Valid values:
  - `EQUAL`
  - `NOT_EQUAL`
  - `LESS_THAN`
  - `LESS_THAN_OR_EQUAL`
  - `GREATER_THAN`
  - `GREATER_THAN_OR_EQUAL`
  - `ARRAY_CONTAINS`
  - `IN`
  - `ARRAY_CONTAINS_ANY`
  - `NOT_IN`
- `value` (String, Required) - The value to compare against, JSON encoded.

#### where_group (Optional)

A group of filter conditions combined with their own operator. Use this to nest AND/OR logic (e.g., `status = "active" AND (role = "admin" OR role = "editor")`).

- `group_operator` (String, Optional) - The operator used to combine conditions within this group. Valid values: `AND` (default), `OR`.
- `where` (Block, Required) - One or more filter conditions within this group. Each has the same schema as the top-level `where` block:
  - `field` (String, Required) - The field path to filter on.
  - `operator` (String, Required) - The comparison operator (same values as top-level `where`).
  - `value` (String, Required) - The value to compare against, JSON encoded.

#### order_by (Optional)

Ordering for the query results.

- `field` (String, Required) - The field path to order by.
- `direction` (String, Optional) - The sort direction. Valid values: `ASCENDING` (default), `DESCENDING`.

### Read-Only

- `documents` (List of Object) - List of documents in the collection. Each document contains:
  - `document_id` (String) - The document ID.
  - `fields` (String) - JSON string of document fields.
  - `create_time` (String) - The time the document was created.
  - `update_time` (String) - The time the document was last updated.
