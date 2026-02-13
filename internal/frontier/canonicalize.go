// B-11: URL canonicalization — normalizes URLs before dedup checks and frontier operations.
// Prevents 5-15% duplicate fetches from URL variants (http vs https, trailing slash,
// fragments, UTM parameters, default ports, uppercase hosts).
package frontier

import (
	"net/url"
	"path"
	"strings"
)

// trackingParams are query parameters that should be stripped during canonicalization.
// These tracking parameters produce distinct Cuckoo filter keys for identical content.
var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"ref":          true,
	"source":       true,
	"fbclid":       true,
	"gclid":        true,
	"mc_cid":       true,
	"mc_eid":       true,
}

// canonicalize normalizes a URL to prevent duplicate fetches of the same content.
// It applies 7 normalization steps:
// 1. Force HTTPS scheme
// 2. Lowercase host
// 3. Remove default ports (443, 80)
// 4. Strip fragment (server never sees it)
// 5. Remove tracking query parameters
// 6. Normalize path (resolve ./ and ../)
// 7. Remove trailing slash (except root)
func canonicalize(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Step 1: Force HTTPS
	u.Scheme = "https"

	// Step 2: Lowercase host
	u.Host = strings.ToLower(u.Host)

	// Step 3: Remove default ports
	if u.Port() == "443" || u.Port() == "80" {
		u.Host = u.Hostname()
	}

	// Step 4: Strip fragment (server never sees it)
	u.Fragment = ""

	// Step 5: Remove tracking query parameters
	q := u.Query()
	for key := range q {
		lowerKey := strings.ToLower(key)
		if trackingParams[lowerKey] || strings.HasPrefix(lowerKey, "utm_") {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()

	// Step 6: Normalize path — resolve ./ and ../
	if u.Path != "" {
		u.Path = path.Clean(u.Path)
	}

	// Step 7: Remove trailing slash (except root / which is meaningful)
	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	return u.String(), nil
}
