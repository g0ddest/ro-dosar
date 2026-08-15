package http

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"

	"ro-dosar/internal/domain"
)

// BrowserClient uses headless Chrome to fetch pages with JavaScript protection
type BrowserClient struct {
	timeout     time.Duration
	allocCtx    context.Context
	allocCancel context.CancelFunc
	taskCtx     context.Context
	taskCancel  context.CancelFunc
	initialized bool
	mu          sync.Mutex
}

// NewBrowserClient creates a new browser-based client
func NewBrowserClient(timeout time.Duration) *BrowserClient {
	return &BrowserClient{
		timeout: timeout,
	}
}

// initBrowserLocked initializes the browser without locking (must be called with lock held)
func (b *BrowserClient) initBrowserLocked(_ context.Context, url string) error {
	if b.initialized {
		return nil
	}

	// Create Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	b.taskCtx, b.taskCancel = chromedp.NewContext(b.allocCtx)

	// Navigate to the base URL to solve the JS challenge
	err := chromedp.Run(b.taskCtx,
		chromedp.Navigate(url),
		chromedp.Sleep(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize browser: %w", err)
	}

	b.initialized = true
	return nil
}

// Close closes the browser session
func (b *BrowserClient) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.taskCancel != nil {
		b.taskCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
	b.initialized = false
}

// FetchPage fetches an HTML page, waiting until a PDF link is visible
func (b *BrowserClient) FetchPage(ctx context.Context, url string) ([]byte, string, error) {
	return b.fetchPage(ctx, url, chromedp.WaitVisible("a[href$='.pdf']", chromedp.ByQuery))
}

// FetchPageDOM fetches an HTML page, waiting only until a PDF link is present
// in the DOM — some listing pages keep their links inside collapsed sections
// that never become visible
func (b *BrowserClient) FetchPageDOM(ctx context.Context, url string) ([]byte, string, error) {
	return b.fetchPage(ctx, url, chromedp.WaitReady("a[href$='.pdf']", chromedp.ByQuery))
}

// fetchPage fetches an HTML page using headless Chrome, blocking on wait
// before capturing the page's outer HTML
func (b *BrowserClient) fetchPage(ctx context.Context, url string, wait chromedp.Action) ([]byte, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.initBrowserLocked(ctx, url); err != nil {
		return nil, "", err
	}

	// Create a context with 1 minute timeout for waiting PDF links
	timeoutCtx, cancel := context.WithTimeout(b.taskCtx, 1*time.Minute)
	defer cancel()

	var html string
	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(url),
		wait,
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch page (timeout waiting for PDF links): %w", err)
	}

	hash := sha256.Sum256([]byte(html))
	return []byte(html), hex.EncodeToString(hash[:]), nil
}

// FetchPDF fetches a PDF file using the browser's established session
func (b *BrowserClient) FetchPDF(ctx context.Context, url string) ([]byte, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.initBrowserLocked(ctx, url); err != nil {
		return nil, "", err
	}

	// Use a unique variable name to avoid collisions (though we now have a lock)
	resultVar := fmt.Sprintf("_pdfResult_%d", time.Now().UnixNano())

	script := fmt.Sprintf(`
		(async function() {
			try {
				const response = await fetch('%s');
				if (!response.ok) {
					window['%s'] = 'ERROR:HTTP ' + response.status;
					return;
				}
				const blob = await response.blob();
				const reader = new FileReader();
				reader.onloadend = () => {
					window['%s'] = reader.result.split(',')[1];
				};
				reader.onerror = () => {
					window['%s'] = 'ERROR:FileReader error';
				};
				reader.readAsDataURL(blob);
			} catch(e) {
				window['%s'] = 'ERROR:' + e.toString();
			}
		})()
	`, url, resultVar, resultVar, resultVar, resultVar)

	// Execute fetch and wait for result variable to be set
	var base64Data string
	var ready bool
	err := chromedp.Run(b.taskCtx,
		chromedp.Evaluate(fmt.Sprintf("window['%s'] = null", resultVar), nil),
		chromedp.Evaluate(script, nil),
		// Wait for the variable to be non-null, ignore the boolean result
		chromedp.Poll(fmt.Sprintf("window['%s'] !== null", resultVar), &ready, chromedp.WithPollingInterval(100*time.Millisecond)),
		// Now fetch the actual string value
		chromedp.Evaluate(fmt.Sprintf("window['%s']", resultVar), &base64Data),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch PDF: %w", err)
	}

	// Clean up global variable
	_ = chromedp.Run(b.taskCtx, chromedp.Evaluate(fmt.Sprintf("delete window['%s']", resultVar), nil))

	if base64Data == "" {
		return nil, "", fmt.Errorf("no PDF data received")
	}

	if len(base64Data) > 6 && base64Data[:6] == "ERROR:" {
		errMsg := base64Data[6:]
		// Check for HTTP 404 specifically
		if strings.Contains(errMsg, "HTTP 404") {
			return nil, "", domain.ErrRemoteFileNotFound
		}
		return nil, "", fmt.Errorf("fetch error: %s", errMsg)
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode PDF data: %w", err)
	}

	return data, "", nil
}

// FetchPageWithHash fetches page and returns content with hash
func (b *BrowserClient) FetchPageWithHash(ctx context.Context, url string) ([]byte, string, error) {
	return b.FetchPage(ctx, url)
}
