// B-09: TLS fingerprint impersonation using utls.
// Go's default crypto/tls produces a unique JA3 hash instantly recognizable
// as non-browser. utls impersonates exact Chrome/Firefox ClientHello signatures.
package antidetect

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
)

// fingerprints contains browser TLS ClientHello configurations to rotate.
// Using real browser fingerprints reduces block rates by 40-70% on Cloudflare.
var fingerprints = []utls.ClientHelloID{
	utls.HelloChrome_120,
	utls.HelloFirefox_120,
	utls.HelloEdge_85,
	utls.HelloRandomizedALPN, // additional entropy option
}

// domainFingerprints tracks per-domain fingerprint assignments for consistency
// within a session.
var (
	domainFPMu  sync.RWMutex
	domainFPMap = make(map[string]utls.ClientHelloID)
)

// GetDomainFingerprint returns a consistent TLS fingerprint for a domain.
// Same domain always gets the same fingerprint within a session.
func GetDomainFingerprint(domain string) utls.ClientHelloID {
	domainFPMu.RLock()
	fp, ok := domainFPMap[domain]
	domainFPMu.RUnlock()

	if ok {
		return fp
	}

	// Assign a random fingerprint for this domain
	fp = fingerprints[rand.Intn(len(fingerprints))]

	domainFPMu.Lock()
	domainFPMap[domain] = fp
	domainFPMu.Unlock()

	return fp
}

// NewUTLSTransport creates an http.Transport that impersonates the given
// browser's TLS ClientHello. The JA3/JA4 hash will match the real browser,
// bypassing TLS-based bot detection.
func NewUTLSTransport(helloID utls.ClientHelloID) *http.Transport {
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("split host:port %q: %w", addr, err)
		}

		// Dial plain TCP connection
		plainConn, err := (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, fmt.Errorf("tcp dial: %w", err)
		}

		// Wrap with utls to impersonate the specified browser
		uConn := utls.UClient(plainConn, &utls.Config{
			ServerName:         host,
			InsecureSkipVerify: false, // always verify TLS certificates
		}, helloID)

		if err := uConn.Handshake(); err != nil {
			plainConn.Close()
			return nil, fmt.Errorf("utls handshake: %w", err)
		}

		return uConn, nil
	}

	return &http.Transport{
		DialTLSContext:      dial,
		MaxIdleConns:        50_000,
		MaxIdleConnsPerHost: 500,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
}

// NewRotatingTransport creates a transport that rotates TLS fingerprints
// per domain for diversity while maintaining per-domain consistency.
func NewRotatingTransport(domain string) *http.Transport {
	fp := GetDomainFingerprint(domain)
	return NewUTLSTransport(fp)
}
