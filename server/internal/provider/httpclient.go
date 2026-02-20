package provider

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// DefaultHTTPClient wraps net/http.Client to implement the provider.HTTPClient interface.
type DefaultHTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a DefaultHTTPClient with the given timeout.
func NewHTTPClient(timeout time.Duration) *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{Timeout: timeout},
	}
}

// Do converts a provider.HTTPRequest to a net/http request, executes it,
// and returns the result as a provider.HTTPResponse.
func (c *DefaultHTTPClient) Do(req *HTTPRequest) (*HTTPResponse, error) {
	httpReq, err := http.NewRequest(req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}
