package spec

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DiscoverConfig holds parameters for spec discovery for a single API.
type DiscoverConfig struct {
	// APIName is the registered short name (used as the cache key).
	APIName string
	// BaseURL is the API's root URL.
	BaseURL string
	// SpecURL, if non-empty, is checked before probing other locations.
	SpecURL string
	// CacheDir is the directory for CBOR spec cache files.
	CacheDir string
	// Version is the running restish version; cache entries with a different
	// version are discarded.
	Version string
	// Transport is used for all HTTP fetches.  nil uses http.DefaultTransport.
	Transport http.RoundTripper
}

// Discover returns the APISpec for an API, using cache when available.
// Discovery order (first success wins, network steps run in parallel):
//  1. CBOR spec cache
//  2. Explicit SpecURL (if configured)
//  3. Link headers from a GET on BaseURL (service-desc / describedby)
//  4. Well-known paths /openapi.json and /openapi.yaml
//  5. BaseURL body itself
func Discover(ctx context.Context, cfg DiscoverConfig, loaders []Loader) (*APISpec, error) {
	// 1. Cache check (synchronous, no network).
	if cfg.CacheDir != "" {
		if entry, ok := readCache(cfg.CacheDir, cfg.APIName, cfg.Version); ok {
			if spec, err := load(entry.ContentType, entry.Raw, loaders); err == nil && spec != nil {
				return spec, nil
			}
		}
	}

	// 2-5. Network discovery (parallel).
	spec, ttl, err := discoverFromNetwork(ctx, cfg, loaders)
	if err != nil {
		return nil, err
	}

	// Cache the result.
	if cfg.CacheDir != "" && spec != nil {
		var expiresAt time.Time
		if ttl > 0 {
			expiresAt = time.Now().Add(ttl)
		} else {
			expiresAt = time.Now().Add(24 * time.Hour)
		}
		entry := &cacheEntry{
			Version:     cfg.Version,
			FetchedAt:   time.Now(),
			ExpiresAt:   expiresAt,
			ContentType: spec.ContentType,
			Raw:         spec.Raw,
		}
		_ = writeCache(cfg.CacheDir, cfg.APIName, entry)
	}

	return spec, nil
}

// discoverFromNetwork runs parallel probes and returns the first valid spec.
func discoverFromNetwork(ctx context.Context, cfg DiscoverConfig, loaders []Loader) (*APISpec, time.Duration, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		spec     *APISpec
		ttl      time.Duration
		err      error
		priority int // 0 = explicit SpecURL (preferred for errors); 1 = heuristic probes
	}

	// Use a large buffer so goroutines never block on send.
	ch := make(chan result, 16)
	var wg sync.WaitGroup

	launch := func(priority int, fn func() (string, []byte, time.Duration, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct, body, ttl, err := fn()
			if err != nil {
				select {
				case ch <- result{err: err, priority: priority}:
				case <-ctx.Done():
				}
				return
			}
			spec, loadErr := load(ct, body, loaders)
			select {
			case ch <- result{spec: spec, ttl: ttl, err: loadErr, priority: priority}:
			case <-ctx.Done():
			}
		}()
	}

	tr := cfg.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}

	// Explicit spec URL (priority 0 — most authoritative error source).
	if cfg.SpecURL != "" {
		u := cfg.SpecURL
		launch(0, func() (string, []byte, time.Duration, error) {
			return fetchBytes(ctx, u, tr)
		})
	}

	// Probe base URL: extract Link headers and try the body itself.
	baseURL := cfg.BaseURL
	launch(1, func() (string, []byte, time.Duration, error) {
		ct, body, ttl, linkURLs, err := fetchWithLinks(ctx, baseURL, tr)
		if err != nil {
			return "", nil, 0, err
		}
		// Launch Link-header candidates as additional probes.
		// Calling launch() from within a goroutine tracked by wg is safe:
		// this goroutine's wg.Done() hasn't been deferred yet, so the
		// counter is still ≥1 when the inner wg.Add(1) is called.
		for _, lu := range linkURLs {
			u := lu
			launch(1, func() (string, []byte, time.Duration, error) {
				return fetchBytes(ctx, u, tr)
			})
		}
		return ct, body, ttl, nil
	})

	// Well-known paths.
	for _, path := range []string{"/openapi.json", "/openapi.yaml"} {
		u := joinURL(cfg.BaseURL, path)
		launch(1, func() (string, []byte, time.Duration, error) {
			return fetchBytes(ctx, u, tr)
		})
	}

	// Close ch once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect errors, preferring lower-priority values (0 = SpecURL is most
	// authoritative). This ensures a 401 from an explicit SpecURL beats a 404
	// from a well-known-path probe, regardless of goroutine arrival order.
	bestErrPriority := int(^uint(0) >> 1) // max int
	var bestErr error
	for r := range ch {
		if r.spec != nil {
			cancel() // stop remaining probes
			return r.spec, r.ttl, nil
		}
		if r.err != nil && r.priority <= bestErrPriority {
			bestErrPriority = r.priority
			bestErr = r.err
		}
	}

	if bestErr != nil {
		return nil, 0, fmt.Errorf("spec discovery failed: %w", bestErr)
	}
	return nil, 0, fmt.Errorf("no API spec found at %s", cfg.BaseURL)
}

// fetchBytes performs a GET and returns content-type, body, cache TTL, and error.
func fetchBytes(ctx context.Context, rawURL string, tr http.RoundTripper) (string, []byte, time.Duration, error) {
	ct, body, ttl, _, err := fetchWithLinks(ctx, rawURL, tr)
	return ct, body, ttl, err
}

// fetchWithLinks performs a GET and also returns parsed Link header spec URLs.
func fetchWithLinks(ctx context.Context, rawURL string, tr http.RoundTripper) (ct string, body []byte, ttl time.Duration, links []string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, 0, nil, err
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		return "", nil, 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, 0, nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, 0, nil, err
	}

	ct = resp.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}

	ttl = cacheTTL(resp)
	links = extractSpecLinks(rawURL, resp.Header)
	return ct, body, ttl, links, nil
}

// cacheTTL extracts the cache duration from a response's Cache-Control header.
func cacheTTL(resp *http.Response) time.Duration {
	cc := resp.Header.Get("Cache-Control")
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			var secs int
			if _, err := fmt.Sscanf(part[8:], "%d", &secs); err == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return 0
}

// linkRel matches a Link header entry and captures the URL and rel value.
// RFC 8288 allows both quoted (rel="next") and unquoted (rel=next) forms.
var linkRel = regexp.MustCompile(`<([^>]+)>[^<]*\brel=(?:"([^"]+)"|([^\s,;]+))`)

// extractSpecLinks parses Link response headers and returns URLs whose rel is
// "service-desc" or "describedby".
func extractSpecLinks(baseURL string, h http.Header) []string {
	var out []string
	for _, header := range h["Link"] {
		for _, match := range linkRel.FindAllStringSubmatch(header, -1) {
			// match[2] = quoted rel value; match[3] = unquoted rel value.
			rel := match[2]
			if rel == "" {
				rel = match[3]
			}
			if rel == "service-desc" || rel == "describedby" {
				u := resolveRef(baseURL, match[1])
				if u != "" {
					out = append(out, u)
				}
			}
		}
	}
	return out
}

// resolveRef resolves ref against base, returning the absolute URL string.
func resolveRef(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return b.ResolveReference(r).String()
}

// joinURL appends path to base, stripping any trailing slash from base.
func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}
