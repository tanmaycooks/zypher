// Package transport provides HTTP transport implementations.
// E-01: HTTP/3 (QUIC) support via Alt-Svc detection.
package transport

import (
	"net/http"
	"strings"
	"sync"

	"github.com/quic-go/quic-go/http3"
)

// MultiplexedTransport transparently upgrades connections from HTTP/1.1 or HTTP/2
// to HTTP/3 (QUIC) when the server advertises H3 support via the Alt-Svc header.
//
// QUIC provides: 0-RTT connection establishment, no TCP head-of-line blocking,
// and ~5% latency improvement on lossy networks.
type MultiplexedTransport struct {
	h1h2      http.RoundTripper
	h3        *http3.Transport
	h3capable sync.Map // domain → bool
}

// NewMultiplexedTransport creates a transport that auto-upgrades to HTTP/3.
func NewMultiplexedTransport(baseTransport http.RoundTripper) *MultiplexedTransport {
	return &MultiplexedTransport{
		h1h2: baseTransport,
		h3:   &http3.Transport{},
	}
}

// RoundTrip implements http.RoundTripper with transparent H3 upgrade.
func (t *MultiplexedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()

	// Check if host is known to support HTTP/3
	if capable, ok := t.h3capable.Load(host); ok && capable.(bool) {
		resp, err := t.h3.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		// HTTP/3 failed — fall back to HTTP/1.1 or HTTP/2
		t.h3capable.Store(host, false)
	}

	// Standard HTTP/1.1 or HTTP/2
	resp, err := t.h1h2.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Check for Alt-Svc header announcing HTTP/3 support
	if altSvc := resp.Header.Get("Alt-Svc"); strings.Contains(altSvc, "h3") {
		t.h3capable.Store(host, true)
	}

	return resp, nil
}

// Close terminates all QUIC connections.
func (t *MultiplexedTransport) Close() error {
	return t.h3.Close()
}
