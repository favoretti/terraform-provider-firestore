---
page_title: "documents_map Function"
description: |-
  Converts a Firestore documents list into a map keyed by document_id.
---

# documents_map (Function)

Takes the `documents` list from a `firestore_documents` data source and returns
a map of `document_id => decoded_fields`. This replaces the 6-line
ternary-for-jsondecode pattern that appears across reader outputs.

## Signature

```hcl
provider::firestore::documents_map(documents: list) -> map(string, dynamic)
```

## Arguments

| Name | Type | Description |
|------|------|-------------|
| `documents` | `list` | The `documents` attribute from a `firestore_documents` data source. Each element must have `document_id` and `fields` attributes. |

## Return Value

A map where keys are document IDs and values are the decoded fields objects.

## Example Usage

### Before

```hcl
output "custom_roles" {
  value = (
    length(data.firestore_documents.custom_roles) > 0
    ? { for doc in data.firestore_documents.custom_roles[0].documents
      : doc.document_id => jsondecode(doc.fields) }
    : {}
  )
}
```

### After

```hcl
output "custom_roles" {
  value = provider::firestore::documents_map(
    try(data.firestore_documents.custom_roles[0].documents, []))
}
```

## Error Conditions

Returns a function error if any element in the list is missing the `document_id`
or `fields` attribute (covers failure mode 27).
