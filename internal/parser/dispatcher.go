package parser

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
	"strings"
)

type ParsedResult struct {
	Links []string // Package parser provides content-type aware parsing dispatch.

	// E-05: Dispatches to the correct parser based on Content-Type header.

	// E-08: goquery-based CSS selector extraction from YAML rules.

	// ParsedResult holds the output of a parse operation.
	// extracted links
	// extracted structured fields
	// detected content type
	// page title (if HTML)
	// main text content
	// Dispatcher routes parsing to the correct handler based on Content-Type.
	//
	// E-05: Supports HTML, JSON, XML, RSS/Atom, CSV, and plain text.
	// Each content type gets a specialized parser that extracts maximum value.

	// domain → rules

	// ExtractionRule defines a CSS selector-based extraction from YAML config.
	// E-08: Pre-compiled at startup from config/extraction_rules.yaml.
	// "text" or HTML attribute name
	// NewDispatcher creates a parser dispatcher with optional extraction rules.

	// Dispatch parses content based on its Content-Type.
	// Normalize content type
	// parseHTML extracts links, title, and structured fields from HTML.
	// Extract title
	// Extract links
	// E-08: Apply domain-specific CSS extraction rules
	// Extract main text content
	// parseJSON extracts links and fields from JSON responses.

	// Attempt to extract links from JSON
	// parseXML extracts links from XML/RSS/Atom feeds.
	// Try parsing as RSS/Atom
	// parseCSV reads CSV data and returns it as structured fields.

	// Extract URLs from CSV cells
	// parsePlainText handles plain text responses.

	// extractLinksFromJSON recursively extracts URLs from JSON structures.
	// extractDomainFromURL extracts the domain from a URL string.
	// DetectContentType auto-detects content type from response headers and sniffing.
	// Fall back to content sniffing
	// default assumption for web pages

	Fields      map[string]string
	ContentType string
	Title       string
	Text        string
}
type Dispatcher struct {
	extractionRules map[string][]ExtractionRule
}
type ExtractionRule struct {
	Name     string `yaml:"name"`
	Selector string `yaml:"selector"`
	Attr     string `yaml:"attr,omitempty"`
}

func NewDispatcher(rules map[string][]ExtractionRule) *Dispatcher {
	if rules == nil {
		rules = make(map[string][]ExtractionRule)
	}
	return &Dispatcher{extractionRules: rules}
}

func (d *Dispatcher) Dispatch(contentType string, body io.Reader, url string) (*ParsedResult, error) {

	mediaType := strings.ToLower(strings.Split(contentType, ";")[0])
	mediaType = strings.TrimSpace(mediaType)

	switch {
	case strings.Contains(mediaType, "html"):
		return d.parseHTML(body, url)
	case strings.Contains(mediaType, "json"):
		return d.parseJSON(body, url)
	case strings.Contains(mediaType, "xml") || strings.Contains(mediaType, "rss") || strings.Contains(mediaType, "atom"):
		return d.parseXML(body, url)
	case strings.Contains(mediaType, "csv"):
		return d.parseCSV(body, url)
	default:
		return d.parsePlainText(body, url)
	}
}

func (d *Dispatcher) parseHTML(body io.Reader, pageURL string) (*ParsedResult, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("goquery parse: %w", err)
	}

	result := &ParsedResult{
		ContentType: "text/html",
		Links:       make([]string, 0),
		Fields:      make(map[