package mirror

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPFetcher makes HTTP requests to fetch data from REST APIs.
type HTTPFetcher struct {
	Client *http.Client
}

// NewHTTPFetcher returns an HTTPFetcher with a default 30s timeout client.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Fetch performs an HTTP request based on the connector's URL, HTTPMethod,
// Headers, and HTTPBody. Returns the response body as JSON.
func (f *HTTPFetcher) Fetch(conn *Connector) (*FetchResult, error) {
	if conn.URL == "" {
		return nil, fmt.Errorf("http fetcher: connector %s has no URL configured", conn.ID)
	}

	method := strings.ToUpper(conn.HTTPMethod)
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if conn.HTTPBody != "" {
		body = bytes.NewBufferString(conn.HTTPBody)
	}

	req, err := http.NewRequest(method, conn.URL, body)
	if err != nil {
		return nil, fmt.Errorf("http fetcher: build request: %w", err)
	}

	// Apply connector-defined headers
	for k, v := range conn.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http fetcher: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("http fetcher: %s %s returned %d: %s", method, conn.URL, resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("http fetcher: read response: %w", err)
	}

	output := bytes.TrimSpace(respBody)

	// Validate it's valid JSON
	if !json.Valid(output) {
		// Wrap non-JSON response as a JSON string
		wrapped, _ := json.Marshal(string(output))
		output = wrapped
	}

	return &FetchResult{
		Data:      json.RawMessage(output),
		FetchedAt: time.Now(),
	}, nil
}