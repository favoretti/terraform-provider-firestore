# terraform-provider-firestore — Agent and Copilot Instructions

## Ways to Fail

Every change to this provider must be checked against the following failure modes. Any new feature or behavior must extend this list with how that feature can fail.

1. **State drift** — Terraform shows a diff after a clean apply with no user changes. Computed attributes such as `update_time` must use `UseStateForUnknown()`.
2. **Data loss on update** — PATCH requests without `updateMask` silently delete Firestore fields not in the Terraform config. All updates must include `updateMask.fieldPaths`.
3. **Silent type corruption** — JSON numbers decoded as `float64` lose precision beyond 2^53. Use `json.Decoder.UseNumber()` and `strconv.ParseInt`. Parse errors must surface as diagnostics, not be discarded.
4. **Missing input validation** — Invalid operator or direction values must be rejected at plan time, not at API call time. Use `stringvalidator.OneOf(...)` in the schema. The `fields` attribute must be validated as a JSON object at plan time. The `collection` attribute must be non-empty (`stringvalidator.LengthAtLeast(1)`). The `limit` attribute must be positive (`int64validator.AtLeast(1)`).
5. **No retry on transient failure** — A single 429, 500, 502, or 503 must not cause permanent failure. All HTTP calls go through `doHTTPRequest`, which retries with exponential backoff up to 4 attempts. Network errors (connection refused) are also retried.
6. **Test protocol mismatch** — Tests must use `ProtoV6ProviderFactories` with `providerserver.NewProtocol6WithError`. Tests must use `ConfigStateChecks` with `statecheck.ExpectKnownValue` and `tfjsonpath`, not the legacy `Check` field. All resource acceptance tests must include `CheckDestroy` to verify backend cleanup.
7. **Unchecked I/O errors** — `io.ReadAll` errors must be checked. All HTTP I/O goes through `doHTTPRequest`, which handles and surfaces these errors.
8. **State removal on transient error** — `Read()` must only call `resp.State.RemoveResource(ctx)` on a confirmed 404. Any other error status must add an error diagnostic and leave state unchanged.
9. **No schema version** — `SchemaVersion` must be set on the resource. Schema changes without a version increment and a matching `StateUpgraders` entry will corrupt existing `.tfstate` files.
10. **Unvalidated API response format** — `doHTTPRequest` validates that 200 responses carry `Content-Type: application/json`. HTML error pages must never be stored as field data.
11. **Silent partial configuration** — If `impersonate_service_account` is set without explicit credentials, `Configure()` emits a warning. If `project` cannot be resolved, `Configure()` emits an error and stops.
12. **Accidental schema modifier removal** — `RequiresReplace` on `collection`, `document_id`, `project`, and `database` must not be removed. `UseStateForUnknown` on `name`, `create_time`, `update_time`, `document_id`, `project`, and `database` must not be removed. `fields` must not gain `RequiresReplace`. Schema-level unit tests enforce these constraints.
13. **Unverified resource destruction** — Acceptance tests must verify that resources are removed from Firestore after Terraform destroys them. All resource `TestCase` structs must include `CheckDestroy`.
14. **Inconsistent map serialization** — `firestoreFieldsToStringMap()` must produce deterministic output. `json.Marshal` on `map[string]interface{}` sorts keys alphabetically, so nested object serialization is stable across plans.
15. **Integer misparse in fields_map** — Firestore returns `integerValue` as a JSON string (`"42"`), but after `json.Unmarshal` without `UseNumber()` it may arrive as `float64`. `firestoreFieldsToStringMap` handles both representations.
16. **Null representation ambiguity** — Null fields and empty-string fields both map to `""` in `fields_map`. Users needing to distinguish null from empty string must use the `fields` JSON attribute with `jsondecode()`.
17. **Infinite pagination loop** — `listDocuments` caps pagination at 100 pages (30,000 documents). If the cap is reached, a warning diagnostic is emitted instead of looping indefinitely.
18. **Silent truncation** — Without pagination, `listDocuments` silently truncates at ~300 documents. With pagination, the warning diagnostic at the 100-page cap replaces silent truncation.
19. **Empty select list** — An empty `select` list would produce a request with no `mask.fieldPaths`, equivalent to omitting it. Use `listvalidator.SizeAtLeast(1)` to reject empty lists at plan time.
20. **Select with non-existent field** — Firestore returns the document with the field absent (no error). This is expected API behavior. Users should check `fields_map` for the presence of expected fields.
21. **Missing map_key field** — If `map_key` is set but a returned document does not contain that field in `fields_map`, `Read()` must emit an error diagnostic and stop.
22. **Empty map_key value** — If `map_key` is set and a document's field value is empty, `Read()` must emit an error diagnostic and stop.
23. **Duplicate map_key value** — If `map_key` is set and two documents share the same field value, `Read()` must emit an error diagnostic identifying both document IDs.
24. **Composite map_key with missing field** — If any field in the `map_key` list is absent from a document's `fields_map`, `Read()` must emit an error diagnostic identifying the document and the missing field. Covered by `resolveDocumentMapKey`.
25. **Composite map_key with empty segment** — If any field value in the composite key is empty, `Read()` must emit an error diagnostic. An empty segment produces ambiguous keys (e.g., `"_prod"` vs `"prod"`). Covered by `resolveDocumentMapKey`.
26. **Composite key collision from separator in values** — If field values contain the separator character, composite keys can collide (e.g., `"a_b" + "c"` vs `"a" + "b_c"` both produce `"a_b_c"`). This is documented behavior, not a runtime error. Users must choose a separator that does not appear in their field values.

## Consistency Requirements

- Every code change must be checked against all twenty-six failure modes before it is committed.
- Every test must be traceable to at least one failure mode. The test name or comment must identify which failure mode it covers.
- Unit tests use `go test ./internal/provider/... -run "^Test[^A]"` (no `TF_ACC`).
- Acceptance tests require `TF_ACC=1`, `GOOGLE_PROJECT`, and `GOOGLE_CREDENTIALS` or `GOOGLE_APPLICATION_CREDENTIALS`.
- All changes must compile and pass `go vet ./...` before commit.
- All resource acceptance tests must include `CheckDestroy`.

## Adding Features

When adding a feature:
1. Identify which failure modes apply to the new feature.
2. Add entries to this list for any new ways the feature can fail.
3. Write tests that cover each new failure mode before writing the implementation (regression-first workflow).
4. Update `go.mod` and run `go mod tidy` if new dependencies are introduced.

## Key Files

| File | Role |
|------|------|
| `internal/provider/document_resource.go` | Document CRUD, field type conversion, schema version |
| `internal/provider/document_data_source.go` | Single-document data source, direct ID lookup, field projection |
| `internal/provider/documents_data_source.go` | Collection data source, where, order_by, limit, pagination, field projection, map_key |
| `internal/provider/helpers.go` | `doHTTPRequest` (retry, Content-Type check), `firestoreFieldsToStringMap` (all types), `jsonStringValidator`, `buildFirestoreWhereClause` |
| `internal/provider/provider.go` | Auth, credential resolution, Configure() |
| `internal/provider/document_resource_test.go` | Resource acceptance tests |
| `internal/provider/document_data_source_test.go` | Single-document data source acceptance tests |
| `internal/provider/documents_data_source_test.go` | Collection data source acceptance tests |
| `internal/provider/helpers_test.go` | Unit tests for helpers and HTTP retry logic |
| `internal/provider/provider_test.go` | Provider schema unit tests and test setup |
