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

# Create a document with explicit ID
resource "firestore_document" "user" {
  collection  = "users"
  document_id = "user-123"
  fields = jsonencode({
    name   = "John Doe"
    email  = "john@example.com"
    age    = 30
    active = true
    role   = "admin"
    tags   = ["admin", "developer"]
    profile = {
      bio      = "Software engineer"
      location = "San Francisco"
    }
  })
}

# Create a document with auto-generated ID
resource "firestore_document" "order" {
  collection = "users/${firestore_document.user.document_id}/orders"
  fields = jsonencode({
    product  = "Widget"
    quantity = 5
    price    = 19.99
  })
}

# Read a single document
data "firestore_document" "user" {
  collection  = "users"
  document_id = firestore_document.user.document_id
}

# List all documents in a collection
data "firestore_documents" "all_users" {
  collection = "users"
}

# Query documents with filters
data "firestore_documents" "active_users" {
  collection = "users"

  where {
    field    = "active"
    operator = "EQUAL"
    value    = jsonencode(true)
  }

  order_by {
    field     = "name"
    direction = "ASCENDING"
  }

  limit = 100
}

# Query with OR filter operator
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

# Query with nested AND/OR logic using where_group:
# active = true AND (role = "admin" OR role = "editor")
data "firestore_documents" "active_privileged_users" {
  collection      = "users"
  filter_operator = "AND"

  where {
    field    = "active"
    operator = "EQUAL"
    value    = jsonencode(true)
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

output "user_fields" {
  value = jsondecode(data.firestore_document.user.fields)
}

output "all_user_ids" {
  value = [for doc in data.firestore_documents.all_users.documents : doc.document_id]
}

output "privileged_user_ids" {
  value = [for doc in data.firestore_documents.active_privileged_users.documents : doc.document_id]
}
