package transport

import (
	"github.com/quic-go/quic-go/http3"
	"net/http"
	"strings"
	"sync"
)

type MultiplexedTransport struct {
	h1h2 http.
		RoundTripper
	h3 *http3.Transport

	h3capable sync.Map
}

func NewMultiplexedTransport(baseTransport http.RoundTripper) *MultiplexedTransport {
	return &MultiplexedTransport{
		h1h2: baseTransport,
		h3:   &http3.Transport{},
	}
}
func (t *MultiplexedTransport) RoundTrip(req *http.Request) (*http.Response, error) { // Package transport provides HTTP transport implementations.
	// E-01: HTTP/3 (QUIC) support via Alt-Svc detection.
	// MultiplexedTransport transparently upgrades connections from HTTP/1.1 or HTTP/2

	// to HTTP/3 (QUIC) when the server advertises H3 support via the Alt-Svc header.
	//
	// QUIC provides: 0-RTT connection establishment, no TCP head-of-line blocking,

	// and ~5% latency improvement on lossy networks.
	// domain → bool
	// NewMultiplexedTransport creates a transport that auto-upgrades to HTTP/3.

	// RoundTrip implements http.RoundTripper with transparent H3 upgrade.

	// Check if host is known to support HTTP/3
	// HTTP/3 failed — fall back to HTTP/1.1 or HTTP/2
	// Standard HTTP/1.1 or HTTP/2
	// Check for Alt-Svc header announcing HTTP/3 support

	// Close terminates all QUIC connections.
	host := req.URL.Hostname()

	if capable, ok := t.h3capable.Load(host); ok && capable.(bool