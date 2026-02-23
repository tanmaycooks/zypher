// Package transport provides the standard HTTP transport with DNS cache integration.
package transport

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/anand/webscrapper/internal/dns"
)

// NewHTTPTransport creates a production-ready HTTP transport with:
// - DNS cache integration for <1ms DNS lookups
// - Connection pooling tuned for high throughput
// - TLS 1.2+ enforcement
func NewHTTPTransport(dnsCache *dns.Cache) *http.Transport {
	transport := &http.Transport{
		DialContext:           dnsCache.DialContext(),
		MaxIdleConns:          50_000,
		MaxIdleConnsPerHost:   500,
		MaxConnsPerHost:       500,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    false, // we handle decompression with pooled readers
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return transport
}
