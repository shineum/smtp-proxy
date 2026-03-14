package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const sesServiceName = "ses"

// SigV4HTTPClient wraps an HTTPClient and signs every request with AWS
// Signature V4 before forwarding it to the inner client.
type SigV4HTTPClient struct {
	inner  HTTPClient
	signer *v4.Signer
	creds  aws.Credentials
	region string
}

// NewSigV4HTTPClient creates a signing wrapper around the given client.
func NewSigV4HTTPClient(inner HTTPClient, accessKeyID, secretAccessKey, region string) *SigV4HTTPClient {
	return &SigV4HTTPClient{
		inner:  inner,
		signer: v4.NewSigner(),
		creds: aws.Credentials{
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
		},
		region: region,
	}
}

// Do signs the request with AWS Signature V4 and forwards it to the inner client.
func (c *SigV4HTTPClient) Do(req *HTTPRequest) (*HTTPResponse, error) {
	// Build a standard http.Request for signing.
	body := req.Body
	if body == nil {
		body = []byte{}
	}
	httpReq, err := http.NewRequest(req.Method, req.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Compute SHA-256 of the payload.
	payloadHash := sha256Hex(body)

	// Sign the request (mutates httpReq headers).
	err = c.signer.SignHTTP(context.Background(), c.creds, httpReq, payloadHash, sesServiceName, c.region, time.Now())
	if err != nil {
		return nil, err
	}

	// Copy signed headers back to our request.
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	for k := range httpReq.Header {
		req.Headers[k] = httpReq.Header.Get(k)
	}

	return c.inner.Do(req)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
