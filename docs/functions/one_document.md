---
page_title: "one_document Function"
description: |-
  Extracts the decoded fields of a single Firestore document from a list.
---

# one_document (Function)

Takes a `documents` list and returns the decoded fields of the single document.
Returns an empty object if the list is empty. Returns a function error if the
list contains more than one document.

## Signature

```hcl
provider::firestore::one_document(documents: list) -> dynamic
```

## Arguments

| Name | Type | Description |
|------|------|-------------|
| `documents` | `list` | The `documents` attribute from a `firestore_documents` data source. Each element must have `fields`. |

## Return Value

A dynamic value representing the decoded fields of the single document, or an
empty object if the list is empty.

## Example Usage

### Before

```hcl
locals {
  organization = try(one([for doc in data.firestore_documents.organizations.documents
    : jsondecode(doc.fields)]), {})
}
```

### After

```hcl
locals {
  organization = provider::firestore::one_document(
    data.firestore_documents.organizations.documents)
}
```

## Error Conditions

- Returns a function error if the list contains more than one document (covers failure mode 29).
- Returns a function error if the document's `fields` attribute contains invalid JSON (covers failure mode 30).
