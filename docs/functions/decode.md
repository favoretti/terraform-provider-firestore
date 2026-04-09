---
page_title: "decode Function"
description: |-
  Decodes a Firestore document's fields JSON string into a dynamic value.
---

# decode (Function)

Decodes a JSON string (as stored in a document's `fields` attribute) into a
Terraform dynamic value. This replaces the common `jsondecode(doc.fields)`
pattern with a semantically named provider function.

## Signature

```hcl
provider::firestore::decode(fields_json: string) -> dynamic
```

## Arguments

| Name | Type | Description |
|------|------|-------------|
| `fields_json` | `string` | JSON string from a Firestore document's `fields` attribute. |

## Return Value

A dynamic value representing the decoded JSON object.

## Example Usage

```hcl
data "firestore_document" "org" {
  collection  = "org"
  document_id = "my-org"
}

locals {
  org = provider::firestore::decode(data.firestore_document.org.fields)
}

output "billing_account" {
  value = local.org.billing_account_id
}
```

## Error Conditions

Returns a function error if the input is not valid JSON (covers failure mode 30).
