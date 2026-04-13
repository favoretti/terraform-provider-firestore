package provider

import (
	"net/http"
	"net/http/httptest"
)

// redirectTransport rewrites all requests to point at the test server.
type redirectTransport struct {
	server *httptest.Server
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

// newTestClient creates a FirestoreClient that routes all HTTP requests to the given test server.
func newTestClient(server *httptest.Server) *FirestoreClient {
	return &FirestoreClient{
		HTTPClient: &http.Client{Transport: &redirectTransport{server: server}},
		Project:    "test-project",
		Database:   "(default)",
	}
}

// firestoreDocumentResponse returns a standard Firestore document JSON response body.
func firestoreDocumentResponse(name string, fields map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":       name,
		"fields":     fields,
		"createTime": "2024-01-01T00:00:00Z",
		"updateTime": "2024-01-02T00:00:00Z",
	}
}
