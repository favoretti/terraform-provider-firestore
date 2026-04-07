package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func firestoreFieldsToStringMap(fields map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range fields {
		fieldMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		sv, ok := fieldMap["stringValue"]
		if !ok {
			continue
		}
		s, ok := sv.(string)
		if !ok {
			continue
		}
		result[k] = s
	}
	return result
}

func buildFirestoreWhereClause(conditions []WhereCondition) map[string]interface{} {
	buildFilter := func(cond WhereCondition) map[string]interface{} {
		var value interface{}
		if err := json.Unmarshal([]byte(cond.Value.ValueString()), &value); err != nil {
			value = cond.Value.ValueString()
		}
		return map[string]interface{}{
			"fieldFilter": map[string]interface{}{
				"field": map[string]interface{}{"fieldPath": cond.Field.ValueString()},
				"op":    cond.Operator.ValueString(),
				"value": convertToFirestoreValue(value),
			},
		}
	}

	if len(conditions) == 1 {
		return buildFilter(conditions[0])
	}

	filters := make([]interface{}, len(conditions))
	for i, cond := range conditions {
		filters[i] = buildFilter(cond)
	}
	return map[string]interface{}{
		"compositeFilter": map[string]interface{}{
			"op":      "AND",
			"filters": filters,
		},
	}
}

// doHTTPRequest executes an HTTP request with exponential backoff retry on 429 and 5xx responses.
// It reads and closes the response body, checks the Content-Type on success, and returns
// the status code and body bytes. The caller is responsible for interpreting the status code.
func doHTTPRequest(ctx context.Context, client *http.Client, method, reqURL string, headers map[string]string, body []byte) (int, []byte, error) {
	const maxAttempts = 4
	backoff := time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
		if err != nil {
			return 0, nil, fmt.Errorf("creating request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxAttempts-1 {
				continue
			}
			return 0, nil, fmt.Errorf("executing request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return 0, nil, fmt.Errorf("reading response body: %w", readErr)
		}

		status := resp.StatusCode
		if status == http.StatusTooManyRequests || (status >= 500 && status < 600) {
			if attempt < maxAttempts-1 {
				continue
			}
		}

		if status == http.StatusOK {
			ct := resp.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				snippet := string(respBody)
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				return status, respBody, fmt.Errorf("expected application/json response, got %q: %s", ct, snippet)
			}
		}

		return status, respBody, nil
	}

	return 0, nil, fmt.Errorf("request failed after %d attempts", maxAttempts)
}
