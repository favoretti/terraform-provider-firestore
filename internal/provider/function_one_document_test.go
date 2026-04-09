package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// one_document returns the decoded fields of a single document.
func TestOneDocumentFunction_singleDoc(t *testing.T) {
	docs := []types.Object{
		buildDocumentObject("doc-1", `{"name":"Alice","document_type":"organization"}`),
	}

	f := &OneDocumentFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, docs)

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// one_document returns an empty object for an empty list.
func TestOneDocumentFunction_emptyList(t *testing.T) {
	f := &OneDocumentFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, []types.Object{})

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Error())
	}
}

// FM 29: one_document receives more than one document.
func TestOneDocumentFunction_multipleDocs(t *testing.T) {
	docs := []types.Object{
		buildDocumentObject("doc-1", `{"name":"Alice"}`),
		buildDocumentObject("doc-2", `{"name":"Bob"}`),
	}

	f := &OneDocumentFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, docs)

	f.Run(ctx, req, resp)

	if resp.Error == nil {
		t.Fatal("expected error for multiple documents")
	}
	if resp.Error.Text != "expected 0 or 1 document, got 2; add a where filter to narrow the results" {
		t.Fatalf("unexpected error message: %s", resp.Error.Text)
	}
}

// FM 29: one_document receives null input.
func TestOneDocumentFunction_nullInput(t *testing.T) {
	f := &OneDocumentFunction{}

	ctx, req, resp := setupNullDynamicTest(t)

	f.Run(ctx, req, resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error for null input: %s", resp.Error.Error())
	}
}

// one_document handles invalid fields JSON.
func TestOneDocumentFunction_invalidJSON(t *testing.T) {
	docs := []types.Object{
		buildDocumentObject("doc-1", `not json`),
	}

	f := &OneDocumentFunction{}
	ctx, req, resp := setupDocumentsMapTest(t, docs)

	f.Run(ctx, req, resp)

	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
