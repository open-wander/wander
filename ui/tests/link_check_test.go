// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ui/handlers"
	"golang.org/x/net/html"
)

// LinkChecker crawls the UI and checks for broken links and missing assets
type LinkChecker struct {
	baseURL string
	client  *http.Client
	visited map[string]bool
	mu      sync.Mutex
	results CheckResults
	logger  hclog.Logger
}

// BrokenLink represents a broken link or asset
type BrokenLink struct {
	URL        string
	SourcePage string
	StatusCode int
	Error      string
}

// CheckResults contains summary statistics
type CheckResults struct {
	TotalPages        int
	TotalLinks        int
	TotalAssets       int
	BrokenLinks       []BrokenLink
	MissingAssets     []BrokenLink
	VisitedPages      []string
	SkippedExternal   int
	SkippedAnchors    int
	SkippedWebSockets int
}

func NewLinkChecker(baseURL string, logger hclog.Logger) *LinkChecker {
	return &LinkChecker{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow redirects but limit to 10
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
		visited: make(map[string]bool),
		logger:  logger,
	}
}

// Check starts the link checking process
func (lc *LinkChecker) Check() CheckResults {
	lc.logger.Info("Starting link check", "baseURL", lc.baseURL)

	// Start crawling from the root
	lc.crawlPage("/ui/")

	// Wait for all goroutines to complete
	time.Sleep(2 * time.Second)

	// Sort results for consistent output
	sort.Slice(lc.results.BrokenLinks, func(i, j int) bool {
		return lc.results.BrokenLinks[i].URL < lc.results.BrokenLinks[j].URL
	})
	sort.Slice(lc.results.MissingAssets, func(i, j int) bool {
		return lc.results.MissingAssets[i].URL < lc.results.MissingAssets[j].URL
	})
	sort.Strings(lc.results.VisitedPages)

	lc.logger.Info("Link check complete",
		"totalPages", lc.results.TotalPages,
		"totalLinks", lc.results.TotalLinks,
		"totalAssets", lc.results.TotalAssets,
		"brokenLinks", len(lc.results.BrokenLinks),
		"missingAssets", len(lc.results.MissingAssets))

	return lc.results
}

// crawlPage fetches a page and extracts all links and assets
func (lc *LinkChecker) crawlPage(path string) {
	// Normalize path
	path = lc.normalizePath(path)

	// Check if already visited
	lc.mu.Lock()
	if lc.visited[path] {
		lc.mu.Unlock()
		return
	}
	lc.visited[path] = true
	lc.mu.Unlock()

	// Skip external URLs, anchors, and WebSockets
	if lc.shouldSkip(path) {
		return
	}

	lc.logger.Debug("Crawling page", "path", path)

	fullURL := lc.baseURL + path
	resp, err := lc.client.Get(fullURL)
	if err != nil {
		lc.addBrokenLink(path, "", 0, err.Error())
		return
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode >= 400 {
		lc.addBrokenLink(path, "", resp.StatusCode, fmt.Sprintf("HTTP %d", resp.StatusCode))
		return
	}

	// Record successful page visit
	lc.mu.Lock()
	lc.results.TotalPages++
	lc.results.VisitedPages = append(lc.results.VisitedPages, path)
	lc.mu.Unlock()

	// Only parse HTML pages
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return
	}

	// Parse HTML
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		lc.logger.Warn("Failed to read response body", "path", path, "error", err)
		return
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		lc.logger.Warn("Failed to parse HTML", "path", path, "error", err)
		return
	}

	// Extract and check links
	links := lc.extractLinks(doc)
	for _, link := range links {
		lc.checkLink(link, path)
	}
}

// normalizePath normalizes a URL path
func (lc *LinkChecker) normalizePath(path string) string {
	// Remove fragment
	if idx := strings.Index(path, "#"); idx != -1 {
		path = path[:idx]
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

// shouldSkip checks if a path should be skipped
func (lc *LinkChecker) shouldSkip(path string) bool {
	// Skip external URLs
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		lc.mu.Lock()
		lc.results.SkippedExternal++
		lc.mu.Unlock()
		return true
	}

	// Skip anchors
	if strings.HasPrefix(path, "#") {
		lc.mu.Lock()
		lc.results.SkippedAnchors++
		lc.mu.Unlock()
		return true
	}

	// Skip WebSocket URLs
	if strings.Contains(path, "/ws") || strings.Contains(path, "/exec/ws") {
		lc.mu.Lock()
		lc.results.SkippedWebSockets++
		lc.mu.Unlock()
		return true
	}

	// Skip mailto and tel links
	if strings.HasPrefix(path, "mailto:") || strings.HasPrefix(path, "tel:") {
		return true
	}

	return false
}

// extractLinks extracts all links and assets from an HTML document
func (lc *LinkChecker) extractLinks(n *html.Node) []string {
	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Extract href attributes (links)
			if n.Data == "a" || n.Data == "link" {
				for _, attr := range n.Attr {
					if attr.Key == "href" && attr.Val != "" {
						links = append(links, attr.Val)
					}
				}
			}
			// Extract src attributes (assets)
			if n.Data == "script" || n.Data == "img" || n.Data == "source" {
				for _, attr := range n.Attr {
					if attr.Key == "src" && attr.Val != "" {
						links = append(links, attr.Val)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return links
}

// checkLink checks if a link or asset is valid
func (lc *LinkChecker) checkLink(link, sourcePage string) {
	// Skip if already checked
	normalized := lc.normalizePath(link)
	if lc.shouldSkip(normalized) {
		return
	}

	lc.mu.Lock()

	// Parse the link
	linkURL, err := url.Parse(link)
	if err != nil {
		lc.addBrokenLinkLocked(link, sourcePage, 0, fmt.Sprintf("Invalid URL: %v", err))
		lc.mu.Unlock()
		return
	}

	// Resolve relative URLs
	if !linkURL.IsAbs() {
		baseURL, _ := url.Parse(lc.baseURL + sourcePage)
		linkURL = baseURL.ResolveReference(linkURL)
	}

	// Skip external URLs
	if linkURL.Host != "" {
		parsedBase, _ := url.Parse(lc.baseURL)
		if linkURL.Host != parsedBase.Host {
			lc.results.SkippedExternal++
			lc.mu.Unlock()
			return
		}
	}

	// Get the path for checking
	checkPath := linkURL.Path
	if linkURL.RawQuery != "" {
		checkPath = checkPath + "?" + linkURL.RawQuery
	}

	// Determine if it's an asset or a page
	isAsset := strings.HasPrefix(checkPath, "/ui/static/")

	// Check if already visited
	if lc.visited[checkPath] {
		lc.mu.Unlock()
		return
	}

	// Increment counters
	if isAsset {
		lc.results.TotalAssets++
	} else {
		lc.results.TotalLinks++
	}

	lc.mu.Unlock()

	// Check the link
	// UI routes intentionally expose GET rather than HEAD, so use the same
	// read-only method a browser follows from an anchor.
	resp, err := lc.client.Get(linkURL.String())

	lc.mu.Lock()
	defer lc.mu.Unlock()

	if err != nil {
		if isAsset {
			lc.addMissingAssetLocked(checkPath, sourcePage, 0, err.Error())
		} else {
			lc.addBrokenLinkLocked(checkPath, sourcePage, 0, err.Error())
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		if isAsset {
			lc.addMissingAssetLocked(checkPath, sourcePage, resp.StatusCode, fmt.Sprintf("HTTP %d", resp.StatusCode))
		} else {
			lc.addBrokenLinkLocked(checkPath, sourcePage, resp.StatusCode, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
		return
	}

	// If it's a page link and not an asset, crawl it
	if !isAsset && strings.HasPrefix(checkPath, "/ui/") {
		lc.mu.Unlock()
		go lc.crawlPage(checkPath)
		lc.mu.Lock()
	}
}

// addBrokenLink adds a broken link to the results (thread-safe)
func (lc *LinkChecker) addBrokenLink(url, sourcePage string, statusCode int, errMsg string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.addBrokenLinkLocked(url, sourcePage, statusCode, errMsg)
}

// addBrokenLinkLocked adds a broken link (assumes lock is held)
func (lc *LinkChecker) addBrokenLinkLocked(url, sourcePage string, statusCode int, errMsg string) {
	lc.results.BrokenLinks = append(lc.results.BrokenLinks, BrokenLink{
		URL:        url,
		SourcePage: sourcePage,
		StatusCode: statusCode,
		Error:      errMsg,
	})
}

// addMissingAssetLocked adds a missing asset (assumes lock is held)
func (lc *LinkChecker) addMissingAssetLocked(url, sourcePage string, statusCode int, errMsg string) {
	lc.results.MissingAssets = append(lc.results.MissingAssets, BrokenLink{
		URL:        url,
		SourcePage: sourcePage,
		StatusCode: statusCode,
		Error:      errMsg,
	})
}

// writeResults writes the results to a markdown file
func writeResults(results CheckResults, outputFile string) error {
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# UI Link Check Results\n\n")
	fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Summary
	fmt.Fprintf(f, "## Summary\n\n")
	fmt.Fprintf(f, "- **Total Pages Crawled**: %d\n", results.TotalPages)
	fmt.Fprintf(f, "- **Total Links Checked**: %d\n", results.TotalLinks)
	fmt.Fprintf(f, "- **Total Assets Checked**: %d\n", results.TotalAssets)
	fmt.Fprintf(f, "- **Broken Links**: %d\n", len(results.BrokenLinks))
	fmt.Fprintf(f, "- **Missing Assets**: %d\n", len(results.MissingAssets))
	fmt.Fprintf(f, "- **External Links Skipped**: %d\n", results.SkippedExternal)
	fmt.Fprintf(f, "- **Anchor Links Skipped**: %d\n", results.SkippedAnchors)
	fmt.Fprintf(f, "- **WebSocket URLs Skipped**: %d\n", results.SkippedWebSockets)

	// Status
	fmt.Fprintf(f, "\n## Status\n\n")
	if len(results.BrokenLinks) == 0 && len(results.MissingAssets) == 0 {
		fmt.Fprintf(f, "✅ **PASSED** - No broken links or missing assets found!\n\n")
	} else {
		fmt.Fprintf(f, "❌ **FAILED** - Found %d broken links and %d missing assets\n\n",
			len(results.BrokenLinks), len(results.MissingAssets))
	}

	// Broken Links
	if len(results.BrokenLinks) > 0 {
		fmt.Fprintf(f, "## Broken Links\n\n")
		fmt.Fprintf(f, "| URL | Source Page | Status Code | Error |\n")
		fmt.Fprintf(f, "|-----|-------------|-------------|-------|\n")
		for _, link := range results.BrokenLinks {
			fmt.Fprintf(f, "| `%s` | `%s` | %d | %s |\n",
				link.URL, link.SourcePage, link.StatusCode, link.Error)
		}
		fmt.Fprintf(f, "\n")
	}

	// Missing Assets
	if len(results.MissingAssets) > 0 {
		fmt.Fprintf(f, "## Missing Assets\n\n")
		fmt.Fprintf(f, "| URL | Source Page | Status Code | Error |\n")
		fmt.Fprintf(f, "|-----|-------------|-------------|-------|\n")
		for _, asset := range results.MissingAssets {
			fmt.Fprintf(f, "| `%s` | `%s` | %d | %s |\n",
				asset.URL, asset.SourcePage, asset.StatusCode, asset.Error)
		}
		fmt.Fprintf(f, "\n")
	}

	// Visited Pages
	fmt.Fprintf(f, "## Pages Crawled\n\n")
	for _, page := range results.VisitedPages {
		fmt.Fprintf(f, "- `%s`\n", page)
	}
	fmt.Fprintf(f, "\n")

	return nil
}

// TestLinkChecker tests all UI links and assets
func TestLinkChecker(t *testing.T) {
	if os.Getenv("WANDER_UI_LINK_CHECK") != "1" {
		t.Skip("set WANDER_UI_LINK_CHECK=1 with a reachable cluster to crawl live UI routes")
	}

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "link-checker-test",
		Level:  hclog.Info,
		Output: os.Stdout,
	})

	// Create test server
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)
	server := httptest.NewServer(router.Handler())
	defer server.Close()

	logger.Info("Test server started", "url", server.URL)

	// Create and run link checker
	checker := NewLinkChecker(server.URL, logger)
	results := checker.Check()

	// Write results to file
	outputFile := os.Getenv("WANDER_UI_LINK_REPORT")
	if outputFile == "" {
		outputFile = "../LINK_CHECK_RESULTS.md"
	}
	if err := writeResults(results, outputFile); err != nil {
		t.Fatalf("Failed to write results: %v", err)
	}

	logger.Info("Results written", "file", outputFile)

	// Print summary
	t.Logf("\n=== Link Check Summary ===")
	t.Logf("Total Pages Crawled: %d", results.TotalPages)
	t.Logf("Total Links Checked: %d", results.TotalLinks)
	t.Logf("Total Assets Checked: %d", results.TotalAssets)
	t.Logf("Broken Links: %d", len(results.BrokenLinks))
	t.Logf("Missing Assets: %d", len(results.MissingAssets))
	t.Logf("External Links Skipped: %d", results.SkippedExternal)
	t.Logf("Anchor Links Skipped: %d", results.SkippedAnchors)
	t.Logf("WebSocket URLs Skipped: %d", results.SkippedWebSockets)

	// Report broken links
	if len(results.BrokenLinks) > 0 {
		t.Logf("\n=== Broken Links ===")
		for _, link := range results.BrokenLinks {
			t.Logf("  %s (from %s) - %s", link.URL, link.SourcePage, link.Error)
		}
	}

	// Report missing assets
	if len(results.MissingAssets) > 0 {
		t.Logf("\n=== Missing Assets ===")
		for _, asset := range results.MissingAssets {
			t.Logf("  %s (from %s) - %s", asset.URL, asset.SourcePage, asset.Error)
		}
	}

	// Fail test if there are broken links or missing assets
	if len(results.BrokenLinks) > 0 || len(results.MissingAssets) > 0 {
		t.Errorf("Found %d broken links and %d missing assets",
			len(results.BrokenLinks), len(results.MissingAssets))
	}
}
