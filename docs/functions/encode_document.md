---
page_title: "encode_document Function"
description: |-
  Encodes a Terraform object into a Firestore document JSON string with metadata.
---

# encode_document (Function)

Takes a Terraform object, a document type string, and a schema version string,
merges the metadata into the object, and returns the JSON-encoded string for use
as a document's `fields` attribute. This replaces the
`jsonencode(merge({document_type, schema_version}, ...))` pattern and prevents
accidental omission of metadata fields.

## Signature

```hcl
provider::firestore::encode_document(
  fields: dynamic,
  document_type: string,
  schema_version: string
) -> string
```

## Arguments

| Name | Type | Description |
|------|------|-------------|
| `fields` | `dynamic` | The Terraform object containing document field values. |
| `document_type` | `string` | The document type identifier (must not be empty). |
| `schema_version` | `string` | The schema version string (must not be empty). |

## Return Value

A JSON-encoded string containing the merged fields with `document_type` and
`schema_version` included.

## Example Usage

### Before

```hcl
resource "firestore_document" "role" {
  for_each    = var.custom_roles
  collection  = "org-iam"
  document_id = each.key
  fields = jsonencode(merge(
    { document_type = "custom-role", schema_version = "1.1" },
    each.value
  ))
}
```

### After

```hcl
resource "firestore_document" "role" {
  for_each    = var.custom_roles
  collection  = "org-iam"
  document_id = each.key
  fields      = provider::firestore::encode_document(each.value, "custom-role", "1.1")
}
```

## Error Conditions

- Returns a function error if `document_type` is empty (covers failure mode 28).
- Returns a function error if `schema_version` is empty (covers failure mode 28).
