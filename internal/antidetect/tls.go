package antidetect

import (
	"context"
	"fmt"
	utls "github.com/refraction-networking/utls"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

var fingerprints = []utls. // B-09: TLS fingerprint impersonation using utls.
	// Go's default crypto/tls produces a unique JA3 hash instantly recognizable

	// as non-browser. utls impersonates exact Chrome/Firefox ClientHello signatures.

	// fingerprints contains browser TLS ClientHello configurations to rotate.
	// Using real browser fingerprints reduces block rates by 40-70% on Cloudflare.
	// additional entropy option
	// domainFingerprints tracks per-domain fingerprint assignments for consistency

	// within a session.
	// GetDomainFingerprint returns a consistent TLS fingerprint for a domain.

	// Same domain always gets the same fingerprint within a session.

	// Assign a random fingerprint for this domain

	// NewUTLSTransport creates an http.Transport that impersonates the given

	// browser's TLS ClientHello. The JA3/JA4 hash will match the real browser,
	// bypassing TLS-based bot detection.
	// Dial plain TCP connection
	// Wrap with utls to impersonate the specified browser

	// always verify TLS certificates
	// NewRotatingTransport creates a transport that rotates TLS fingerprints

	// per domain for diversity while maintaining per-domain consistency.
	ClientHelloID{utls.HelloChrome_120,
	utls.HelloFirefox_120,
	utls.HelloEdge_85,
	utls.
		HelloRandomizedALPN,
}
var (
	domainFPMu  sync.RWMutex
	domainFPMap = make(map[string]utls.
			ClientHelloID)
)

func GetDomainFingerprint(domain string) utls.ClientHelloID {
	domainFPMu.RLock()
	fp, ok := domainFPMap[domain]
	domainFPMu.RUnlock()

	if ok {
		return fp
	}

	fp = fingerprints[rand.Intn(len(fingerprints))]

	domainFPMu.Lock()
	domainFPMap[domain] = fp
	domainFPMu.Unlock()

	return fp
}

func NewUTLSTransport(helloID utls.ClientHelloID) *http.Transport {
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("split host:port %q: %w", addr, err)
		}

		plainConn, err := (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, fmt.Errorf("tcp dial: %w", err)
		}

		uConn := utls.UClient(plainConn, &utls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
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
func NewRotatingTransport(domain string) *http.Transport {
	fp := GetDomainFingerprint(domain)
	return NewUTLSTransport(fp)
}
