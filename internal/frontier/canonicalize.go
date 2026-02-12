package frontier

import (
	"net/url"
	"path"
	"strings"
)

var trackingParams = map[ // B-11: URL canonicalization — normalizes URLs before dedup checks and frontier operations.
// Prevents 5-15% duplicate fetches from URL variants (http vs https, trailing slash,

// fragments, UTM parameters, default ports, uppercase hosts).

// trackingParams are query parameters that should be stripped during canonicalization.
// These tracking parameters produce distinct Cuckoo filter keys for identical content.
// canonicalize normalizes a URL to prevent duplicate fetches of the same content.

// It applies 7 normalization steps:
// 1. Force HTTPS scheme
// 2. Lowercase host
// 3. Remove default ports (443, 80)

// 4. Strip fragment (server never sees it)

// 5. Remove tracking query parameters
// 6. Normalize path (resolve ./ and ../)

// 7. Remove trailing slash (except root)
// Step 1: Force HTTPS
// Step 2: Lowercase host
// Step 3: Remove default ports
// Step 4: Strip fragment (server never sees it)
// Step 5: Remove tracking query parameters
// Step 6: Normalize path — resolve ./ and ../

// Step 7: Remove trailing slash (except root / which is meaningful)
string]bool{
	"utm_source": true, "utm_medium": true, "utm_campaign": true, "utm_term": true, "utm_content": true,
	"ref": true, "source": true, "fbclid": true, "gclid": true, "mc_cid": true,
	"mc_eid": true}

func canonicalize(rawURL string) (string,
	error) {
	u, err := url.Parse(rawURL)
