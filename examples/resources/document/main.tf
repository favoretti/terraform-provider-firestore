terraform {
  required_providers {
    firestore = {
      source = "favoretti/firestore"
    }
  }
}

provider "firestore" {
  project = "my-gcp-project"
}

# Create a document with explicit ID
resource "firestore_document" "user" {
  for_each = {
    user-123 = {
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
    }
    user-456 = {
      name   = "Jane Doe"
      email  = "jane@example.com"
      age    = 25
      active = false
      role   = "admin"
      tags   = ["admin", "developer"]
      profile = {
        bio      = "Software engineer"
        location = "San Francisco"
      }
    }
    user-789 = {
      name   = "Ola Nordmann"
      email  = "ola@example.com"
      age    = 40
      active = true
      role   = "admin"
      tags   = ["admin", "operator"]
      profile = {
        bio      = "Systems Administrator"
        location = "Oslo"
      }
    }
    user-abc = {
      name   = "Kari Nordmann"
      email  = "kari@example.com"
      age    = 21
      active = true
      role   = "editor"
      tags   = ["editor", "operator"]
      profile = {
        bio      = "Junior Systems Engineer"
        location = "Oslo"
      }
    }
  }
  collection  = "users"
  document_id = each.key
  fields      = jsonencode(each.value)
}

# Create a document with auto-generated ID
resource "firestore_document" "order" {
  collection = "users/${firestore_document.user["user-456"].document_id}/orders"
  fields = jsonencode({
    product  = "Widget"
    quantity = 5
    price    = 19.99
  })
}

# Read a single document
data "firestore_document" "user" {
  collection  = "users"
  document_id = "user-123"
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

output "all_user_emails" {
  value = [for doc in data.firestore_documents.all_users.documents : jsondecode(doc.fields).email]
}

output "privileged_user_emails" {
  value = [for doc in data.firestore_documents.active_privileged_users.documents : jsondecode(doc.fields).email]
}
