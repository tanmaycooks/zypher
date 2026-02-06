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
	domainFPMu.Unlock(