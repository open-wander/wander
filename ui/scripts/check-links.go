// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/net/html"
)

// LinkChecker crawls the UI and checks for broken links and missing assets
type LinkChecker struct {
	baseURL     string
	client      *http.Client
	visited     map[string]bool
	mu          sync.Mutex
	brokenLinks []BrokenLink
	results     CheckResults
	logger      hclog.Logger
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

	// Remove query string for visited tracking (but keep for checking)
	// We'll use the full URL for checking but normalize for visited tracking
	if idx := strings.Index(path, "?"); idx != -1 {
		// Keep query strings for now
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
	defer lc.mu.Unlock()

	// Parse the link
	linkURL, err := url.Parse(link)
	if err != nil {
		lc.addBrokenLinkLocked(link, sourcePage, 0, fmt.Sprintf("Invalid URL: %v", err))
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
		return
	}

	// Increment counters
	if isAsset {
		lc.results.TotalAssets++
	} else {
		lc.results.TotalLinks++
	}

	// Check the link
	lc.mu.Unlock() // Unlock before making HTTP request
	resp, err := lc.client.Head(linkURL.String())
	lc.mu.Lock() // Re-lock for result recording

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
	f, err := os.Create(outputFile) // #nosec G304 -- outputFile is an explicit trusted CLI destination, not remote input.
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

func main() {
	outputFile := flag.String("output", "LINK_CHECK_RESULTS.md", "Output file for results")
	logLevel := flag.String("log-level", "info", "Log level (trace, debug, info, warn, error)")
	baseURL := flag.String("base-url", "http://localhost:3000", "Base URL to check (must be running server)")
	flag.Parse()

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "link-checker",
		Level: hclog.LevelFromString(*logLevel),
		Color: hclog.AutoColor,
	})

	serverURL := *baseURL

	// Check if server is running
	logger.Info("Checking if server is available", "url", serverURL)
	resp, err := http.Get(serverURL + "/ui/")
	if err != nil {
		logger.Error("Server is not running. Please start the dev server first with: make dev", "error", err)
		fmt.Fprintf(os.Stderr, "\nERROR: Server is not running at %s\n", serverURL)
		fmt.Fprintf(os.Stderr, "Please start the dev server first:\n")
		fmt.Fprintf(os.Stderr, "  cd ui-htmx && make dev\n\n")
		os.Exit(1)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Error("Server returned non-200 status", "statusCode", resp.StatusCode)
		os.Exit(1)
	}

	logger.Info("Server is available")

	logger.Info("Checking links", "baseURL", serverURL)

	// Create and run link checker
	checker := NewLinkChecker(serverURL, logger)
	results := checker.Check()

	// Write results
	logger.Info("Writing results", "file", *outputFile)
	if err := writeResults(results, *outputFile); err != nil {
		logger.Error("Failed to write results", "error", err)
		os.Exit(1)
	}

	// Exit with error if there are broken links or missing assets
	if len(results.BrokenLinks) > 0 || len(results.MissingAssets) > 0 {
		logger.Error("Link check failed",
			"brokenLinks", len(results.BrokenLinks),
			"missingAssets", len(results.MissingAssets))
		os.Exit(1)
	}

	logger.Info("Link check passed!")
}
