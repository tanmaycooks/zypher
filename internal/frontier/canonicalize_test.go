package frontier

import (
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "http to https",
			input:    "http://example.com/page",
			expected: "https://example.com/page",
		},
		{
			name:     "uppercase host",
			input:    "https://EXAMPLE.COM/page",
			expected: "https://example.com/page",
		},
		{
			name:     "remove default HTTPS port",
			input:    "https://example.com:443/page",
			expected: "https://example.com/page",
		},
		{
			name:     "remove trailing slash",
			input:    "https://example.com/page/",
			expected: "https://example.com/page",
		},
		{
			name:     "strip fragment",
			input:    "https://example.com/page#section",
			expected: "https://example.com/page",
		},
		{
			name:     "remove utm params keep others",
			input:    "https://example.com/page?utm_source=twitter&q=foo",
			expected: "https://example.com/page?q=foo",
		},
		{
			name:     "normalize path",
			input:    "https://example.com/a/../b/./c",
			expected: "https://example.com/b/c",
		},
		{
			name:     "remove default HTTP port",
			input:    "http://example.com:80/page",
			expected: "https://example.com/page",
		},
		{
			name:     "root path preserved",
			input:    "https://example.com/",
			expected: "https://example.com/",
		},
		{
			name:     "remove fbclid tracking param",
			input:    "https://example.com/page?fbclid=abc123&id=42",
			expected: "https://example.com/page?id=42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := canonicalize(tt.input)
			if err != nil {
				t.Fatalf("canonicalize(%q) error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("canonicalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.example.com/page", "example.com"},
		{"https://example.com/page", "example.com"},
		{"https://sub.example.com/page", "sub.example.com"},
		{"invalid-url", ""},
	}

	for _, tt := range tests {
		result := extractDomain(tt.url)
		if result != tt.expected {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.url, result, tt.expected)
		}
	}
}
