package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for fetching web resources
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// ClientConfig holds configuration for the HTTP client
type ClientConfig struct {
	Timeout   time.Duration
	UserAgent string
}

// DefaultClientConfig returns the default client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:   30 * time.Second,
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}

// NewClient creates a new HTTP client
func NewClient(config ClientConfig) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		userAgent: config.UserAgent,
	}
}

// FetchResult contains the result of an HTTP fetch operation
type FetchResult struct {
	Body        []byte
	ContentType string
	StatusCode  int
	Hash        string
}

// Fetch retrieves content from a URL
func (c *Client) Fetch(ctx context.Context, url string) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	hash := sha256.Sum256(body)

	return &FetchResult{
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
		Hash:        hex.EncodeToString(hash[:]),
	}, nil
}

// FetchPage fetches an HTML page
func (c *Client) FetchPage(ctx context.Context, url string) ([]byte, error) {
	result, err := c.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

// FetchPDF fetches a PDF file and returns its content and hash
func (c *Client) FetchPDF(ctx context.Context, url string) ([]byte, string, error) {
	result, err := c.Fetch(ctx, url)
	if err != nil {
		return nil, "", err
	}
	return result.Body, result.Hash, nil
}
