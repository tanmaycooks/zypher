# Web Scraper Project - Bug Report & Code Review

**Date:** February 24, 2026  
**Project:** Web Scraper  
**Status:** Analysis Complete

---

## Summary

The codebase is well-structured with multiple production-grade components including circuit breakers, DNS caching, proxy pools, and distributed deduplication. All unit tests pass. However, several issues were identified ranging from potential bugs to missing features and code quality concerns.

---

## Critical Issues

### 1. Division by Zero in Proxy Pool (P0)

**File:** `internal/proxy/pool.go:60`

```go
avgLatency := p.totalMs / float64(total)
```

**Problem:** If `total` is 0 (first request ever), this causes a panic due to division by zero.

**Recommendation:** Add a guard:
```go
if total > 0 {
    avgLatency = p.totalMs / float64(total)
}
```

---

### 2. TLS Fingerprinting Not Being Used (P0)

**File:** `cmd/web/main.go`

**Problem:** The project includes sophisticated TLS fingerprint impersonation in `internal/antidetect/tls.go` using the `utls` library to mimic browser ClientHello signatures. However, `cmd/web/main.go` uses the standard HTTP transport (`transport.NewHTTPTransport()`) instead of the UTLS transport, making the anti-detection code unused.

**Current (line 82-92 in cmd/web/main.go):**
```go
dnsCache := dns.New(5 * time.Minute)
httpTransport := transport.NewHTTPTransport(dnsCache)
httpClient := &http.Client{
    Transport: httpTransport,
    ...
}
```

**Recommendation:** Implement a flag to choose between standard and UTLS transport, or integrate UTLS for better bot detection avoidance.

---

## High Priority Issues

### 3. Redundant Custom String Functions (P1)

**File:** `cmd/scraper/main.go:169-213`

**Problem:** The code implements custom `split()`, `splitAndTrim()`, `indexOf()`, and `trim()` functions that duplicate Go's standard library functionality.

```go
// These are redundant - use strings.Split() and strings.TrimSpace()
func split(s, sep string) []string { ... }
func splitAndTrim(s, sep string) []string { ... }
func indexOf(s, sub string) int { ... }
func trim(s string) string { ... }
```

**Recommendation:** Replace with standard library:
```go
seeds := strings.Split(*seedURLs, ",")
for _, seed := range seeds {
    seed = strings.TrimSpace(seed)
    if seed != "" {
        // ...
    }
}
```

---

### 4. Parser Nil Check Missing (P1)

**File:** `cmd/scraper/main.go:96`

```go
parserDispatch := parser.NewDispatcher(nil)
```

**Problem:** The parser dispatcher is initialized with nil and passed to the worker pool without validation. Need to verify how the worker handles a nil parser.

**Recommendation:** Add nil check or initialize with proper configuration.

---

### 5. DNS Cache Race Condition (P1)

**File:** `internal/dns/cache.go:86-95`

```go
func (c *Cache) refresh(host string) {
    c.mu.Lock()
    if e, ok := c.store[host]; ok {
        e.refreshing = true
    }
    c.mu.Unlock()

    // resolve() will repopulate the store on success
    c.resolve(context.Background(), host)
}
```

**Problem:** If the entry is deleted from the store between the unlock and `resolve()`, the refresh won't update anything. Additionally, the `refreshing` flag is set but never reset after refresh completes.

**Recommendation:** Add proper flag reset after refresh and handle the case where the entry no longer exists.

---

## Medium Priority Issues

### 6. No Redis Reconnection Logic (P2)

**Problem:** If Redis becomes unavailable after initial connection verification, the entire scraper will crash. There's no reconnection strategy or graceful degradation.

**Recommendation:** Implement Redis reconnection with exponential backoff, or add circuit breaker pattern for Redis operations.

---

### 7. HTTP Redirect Following Limited (P2)

**File:** `cmd/web/main.go:86-91`

```go
CheckRedirect: func(req *http.Request, via []*http.Request) error {
    if len(via) >= 5 {
        return fmt.Errorf("too many redirects")
    }
    return nil
},
```

**Problem:** The limit of 5 redirects is hardcoded. Some legitimate sites may require more. This should be configurable.

**Recommendation:** Make redirect limit configurable via environment variable.

---

### 8. Missing Input Validation (P2)

**File:** `cmd/web/main.go`

**Problem:** 
- No URL scheme validation before scraping (handled implicitly but could be more explicit)
- No rate limiting on API endpoints
- No request size limits on POST bodies

**Recommendation:** Add explicit validation and rate limiting middleware.

---

## Low Priority / Code Quality

### 9. Hardcoded Values

| Location | Issue |
|----------|-------|
| `internal/transport/http.go:19-21` | MaxIdleConns=50000, MaxConnsPerHost=500 are very high for single-instance scraper |
| `cmd/web/main.go:338` | Hardcoded concurrency limit of 5 for scraping |
| `internal/proxy/pool.go:67` | EWMA alpha=0.3 hardcoded, not configurable |

---

### 10. Missing Error Handling in Frontend

**File:** `cmd/web/static/app.js`

**Problem:** No error handling for network failures in the UI. Failed requests show no feedback to users.

**Recommendation:** Add error state handling and user-friendly error messages.

---

### 11. Missing Health Checks

**Problem:** Only `/health` endpoint exists in scraper. The web server (`cmd/web/main.go`) has no health check endpoint.

**Recommendation:** Add `/health` endpoint to web server for container orchestration compatibility.

---

## Previously Fixed Bugs (Documented in Code)

The codebase contains comments referencing previously fixed bugs:

| Bug ID | Description | Status |
|--------|-------------|--------|
| B-01 | RedisBloom Cuckoo filter for dedup | Fixed |
| B-02 | DNS cache implementation | Fixed |
| B-03 | Frontier priority score zero-time bug | Fixed |
| B-04 | Adaptive limiter semaphore race | Fixed |
| B-06 | Circuit breaker HALF-OPEN probe gating | Fixed |
| B-09 | TLS fingerprint impersonation | Implemented (but not used) |
| B-13 | Proxy pool O(n log n) to O(1) | Fixed |
| B-17 | UA weight sum from 0.92 to 1.0 | Fixed |

---

## Test Coverage

| Package | Status |
|---------|--------|
| internal/antidetect | ✅ Passing |
| internal/breaker | ✅ Passing |
| internal/dns | ✅ Passing |
| internal/frontier | ✅ Passing |
| internal/limiter | ✅ Passing |
| internal/proxy | ✅ Passing |

---

## Recommendations Priority List

1. **Immediate:** Fix division by zero in proxy pool
2. **Immediate:** Integrate UTLS transport or remove unused code
3. **High:** Remove redundant string functions
4. **High:** Add parser nil validation
5. **Medium:** Implement Redis reconnection logic
6. **Medium:** Add health check to web server
7. **Low:** Make hardcoded values configurable

---

## Conclusion

The codebase is production-ready with good test coverage. The main concerns are:
- Unused TLS fingerprinting capability
- One potential panic (division by zero)
- Redundant code that could be simplified

All issues are fixable with minimal changes.
