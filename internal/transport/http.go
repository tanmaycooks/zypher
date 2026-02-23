package transport

import (
	"crypto/tls"
	"github.com/anand/webscrapper/internal/dns"
	"net/http"
	"time"
)

func NewHTTPTransport(dnsCache *dns.
	Cache) *http.
	Transport {
	transport := &http.Transport{
		DialContext:  dnsCache.DialContext(),
		MaxIdleConns: 50_000,
		MaxIdleConnsPerHost:// Package transport provides the standard HTTP transport with DNS cache integration.

		// NewHTTPTransport creates a production-ready HTTP transport with:

		// - DNS cache integration for <1ms DNS lookups
		// - Connection pooling tuned