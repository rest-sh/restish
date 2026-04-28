package spec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rest-sh/restish/v2/internal/hypermedia"
	"go.yaml.in/yaml/v3"
)

// maxSpecBytes caps the body read during spec discovery (50 MiB).
// OpenAPI specs are rarely larger than a few megabytes; this prevents an
// untrusted server from exhausting memory during api configure / api sync.
const maxSpecBytes = 50 * 1024 * 1024

const defaultDiscoverTimeout = 30 * time.Second

var ErrNoSpecFound = errors.New("no API spec found")

var errNoSpecCandidate = errors.New("no spec at candidate URL")

var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// DiscoverConfig holds parameters for spec discovery for a single API.
type DiscoverConfig struct {
	// APIName is the registered short name (used as the cache key).
	APIName string
	// BaseURL is the API's root URL.
	BaseURL string
	// SpecURL, if non-empty, is checked before probing other locations.
	// Ignored when SpecFiles is set.
	SpecURL string
	// SpecFiles, when non-empty, is an ordered list of local file paths or
	// URLs to load the spec from. Multiple files are deep-merged in order
	// (later entries win on conflict). Network discovery is skipped entirely.
	SpecFiles []string
	// CacheDir is the directory for CBOR spec cache files.
	CacheDir string
	// OperationBase overrides operation URL generation and is included in the
	// cached operation metadata key.
	OperationBase string
	// ServerVariables supplies explicit OpenAPI server variable values and is
	// included in cached operation metadata keys.
	ServerVariables map[string]string
	// Version is the running restish version; cache entries with a different
	// version are discarded.
	Version string
	// Transport is used for all HTTP fetches.  nil uses http.DefaultTransport.
	Transport http.RoundTripper
	// AllowCrossOrigin permits Link-header-discovered spec URLs on other hosts.
	// When false, only same-host discovered links are followed.
	AllowCrossOrigin bool
	// ForceRefresh bypasses any cached entry and rebuilds it from the source.
	ForceRefresh bool
}

// Discover returns the APISpec for an API, using cache when available.
// Discovery order (first success wins, network steps run in parallel):
//  1. CBOR spec cache
//  2. Explicit SpecURL (if configured)
//  3. Link headers from a GET on BaseURL (service-desc / describedby)
//  4. Well-known paths /openapi.json and /openapi.yaml
//  5. BaseURL body itself
func Discover(ctx context.Context, cfg DiscoverConfig, loaders []Loader) (*APISpec, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultDiscoverTimeout)
		defer cancel()
	}

	// 1. Cache check (synchronous, no network).
	if cfg.CacheDir != "" && !cfg.ForceRefresh {
		if entry, ok := readCache(cfg.CacheDir, cfg.APIName, cfg.Version); ok {
			if specFilesChangedSince(cfg.SpecFiles, entry.FetchedAt) {
				goto loadFresh
			}
			opts := entry.loadOptions()
			opts.Context = ctx
			opts.Transport = effectiveTransport(cfg)
			if spec, err := loadWithOptions(entry.contentType(), entry.raw(), loaders, opts); err == nil && spec != nil {
				return spec, nil
			}
		}
	}

loadFresh:
	// 2. Explicit spec files (local paths or URLs) bypass all network probing.
	if len(cfg.SpecFiles) > 0 {
		spec, err := loadSpecFiles(ctx, cfg, loaders)
		if err != nil {
			return nil, err
		}
		if spec != nil && cfg.CacheDir != "" {
			opts := OperationOptions{BaseURL: cfg.BaseURL, OperationBase: cfg.OperationBase, ServerVariables: cfg.ServerVariables}
			set, _ := spec.OperationSetWithOptions(opts)
			entry := &cacheEntry{
				Version:   cfg.Version,
				FetchedAt: time.Now(),
				ExpiresAt: time.Now().Add(24 * time.Hour),
				Spec: cachedRaw{
					ContentType:      spec.ContentType,
					Raw:              spec.Raw,
					SourceURL:        spec.SourceURL,
					LocalPath:        spec.LocalPath,
					AllowCrossOrigin: spec.AllowCrossOrigin,
				},
			}
			if set.Operations != nil {
				entry.upsertOperationSetWithOptions(opts, set)
			}
			_ = writeCache(cfg.CacheDir, cfg.APIName, entry)
		}
		return spec, nil
	}

	// 3-6. Network discovery (parallel).
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
		opts := OperationOptions{BaseURL: cfg.BaseURL, OperationBase: cfg.OperationBase, ServerVariables: cfg.ServerVariables}
		set, _ := spec.OperationSetWithOptions(opts)
		entry := &cacheEntry{
			Version:   cfg.Version,
			FetchedAt: time.Now(),
			ExpiresAt: expiresAt,
			Spec: cachedRaw{
				ContentType:      spec.ContentType,
				Raw:              spec.Raw,
				SourceURL:        spec.SourceURL,
				LocalPath:        spec.LocalPath,
				AllowCrossOrigin: spec.AllowCrossOrigin,
			},
		}
		if set.Operations != nil {
			entry.upsertOperationSetWithOptions(opts, set)
		}
		_ = writeCache(cfg.CacheDir, cfg.APIName, entry)
	}

	return spec, nil
}

func specFilesChangedSince(specFiles []string, fetchedAt time.Time) bool {
	if fetchedAt.IsZero() {
		return false
	}
	for _, src := range specFiles {
		if !isLocalPath(src) {
			continue
		}
		path, err := localPathFromSource(src)
		if err != nil {
			return true
		}
		info, err := os.Stat(path)
		if err != nil || info.ModTime().After(fetchedAt) {
			return true
		}
	}
	return false
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
	tr := effectiveTransport(cfg)

	launch := func(priority int, sourceURL string, fn func() (string, []byte, time.Duration, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct, body, ttl, err := fn()
			if err != nil {
				if priority > 0 && errors.Is(err, errNoSpecCandidate) {
					return
				}
				select {
				case ch <- result{err: err, priority: priority}:
				case <-ctx.Done():
				}
				return
			}
			spec, loadErr := loadWithOptions(ct, body, loaders, LoadOptions{
				Context:          ctx,
				SourceURL:        sourceURL,
				AllowCrossOrigin: cfg.AllowCrossOrigin,
				Transport:        tr,
			})
			select {
			case ch <- result{spec: spec, ttl: ttl, err: loadErr, priority: priority}:
			case <-ctx.Done():
			}
		}()
	}

	// Explicit spec URL (priority 0 — most authoritative error source).
	if cfg.SpecURL != "" {
		u := cfg.SpecURL
		launch(0, u, func() (string, []byte, time.Duration, error) {
			ct, body, ttl, err := fetchBytes(ctx, u, tr)
			if errors.Is(err, errNoSpecCandidate) {
				return "", nil, 0, fmt.Errorf("GET %s: 404 Not Found", u)
			}
			return ct, body, ttl, err
		})
	}

	// Probe base URL: extract Link headers and try the body itself.
	baseURL := cfg.BaseURL
	launch(1, baseURL, func() (string, []byte, time.Duration, error) {
		ct, body, ttl, linkURLs, err := fetchWithLinks(ctx, baseURL, tr, cfg.AllowCrossOrigin)
		if err != nil {
			return "", nil, 0, err
		}
		// Launch Link-header candidates as additional probes.
		// Calling launch() from within a goroutine tracked by wg is safe:
		// this goroutine's wg.Done() hasn't been deferred yet, so the
		// counter is still ≥1 when the inner wg.Add(1) is called.
		for _, lu := range linkURLs {
			u := lu
			launch(1, u, func() (string, []byte, time.Duration, error) {
				return fetchBytes(ctx, u, tr)
			})
		}
		return ct, body, ttl, nil
	})

	// Well-known paths.
	for _, path := range []string{"/openapi.json", "/openapi.yaml"} {
		u := joinURL(cfg.BaseURL, path)
		launch(1, u, func() (string, []byte, time.Duration, error) {
			return fetchBytes(ctx, u, tr)
		})
	}

	// Close ch once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect errors, preferring lower-priority values (0 = SpecURL is most
	// authoritative). Same-priority errors are joined so all causes are visible.
	bestErrPriority := math.MaxInt
	var bestErrs []error
	for r := range ch {
		if r.spec != nil {
			cancel() // stop remaining probes
			return r.spec, r.ttl, nil
		}
		if r.err != nil {
			switch {
			case r.priority < bestErrPriority:
				bestErrPriority = r.priority
				bestErrs = []error{r.err}
			case r.priority == bestErrPriority:
				bestErrs = append(bestErrs, r.err)
			}
		}
	}

	if len(bestErrs) > 0 {
		return nil, 0, fmt.Errorf("spec discovery failed: %w", errors.Join(bestErrs...))
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return nil, 0, fmt.Errorf("%w at %s", ErrNoSpecFound, cfg.BaseURL)
}

// fetchBytes performs a GET and returns content-type, body, cache TTL, and error.
func fetchBytes(ctx context.Context, rawURL string, tr http.RoundTripper) (string, []byte, time.Duration, error) {
	ct, body, ttl, _, err := fetchWithLinks(ctx, rawURL, tr, true)
	return ct, body, ttl, err
}

// fetchWithLinks performs a GET and also returns parsed Link header spec URLs.
func fetchWithLinks(ctx context.Context, rawURL string, tr http.RoundTripper, allowCrossOrigin bool) (ct string, body []byte, ttl time.Duration, links []string, err error) {
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
		if resp.StatusCode == http.StatusNotFound {
			return "", nil, 0, nil, errNoSpecCandidate
		}
		return "", nil, 0, nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}

	body, err = io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes+1))
	if err != nil {
		return "", nil, 0, nil, err
	}
	if int64(len(body)) > maxSpecBytes {
		return "", nil, 0, nil, fmt.Errorf("spec body from %s exceeds limit of %d bytes", rawURL, maxSpecBytes)
	}

	ct = resp.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}

	ttl = cacheTTL(resp)
	links = filterDiscoveredSpecLinks(rawURL, extractSpecLinks(rawURL, resp.Header), allowCrossOrigin)
	return ct, body, ttl, links, nil
}

// effectiveTransport returns cfg.Transport when set, or http.DefaultTransport.
func effectiveTransport(cfg DiscoverConfig) http.RoundTripper {
	if cfg.Transport != nil {
		return cfg.Transport
	}
	return http.DefaultTransport
}

// cacheTTL extracts the cache duration from a response's Cache-Control header.
func cacheTTL(resp *http.Response) time.Duration {
	cc := resp.Header.Get("Cache-Control")
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			if secs, err := strconv.Atoi(part[8:]); err == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return 0
}

// extractSpecLinks parses Link response headers and returns URLs whose rel is
// "service-desc" or "describedby".
func extractSpecLinks(baseURL string, h http.Header) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	var out []string
	for _, parsed := range hypermedia.LinkHeaderLinks(base, h) {
		if parsed.Rel == "service-desc" || parsed.Rel == "describedby" {
			out = append(out, parsed.URI)
		}
	}
	return out
}

func filterDiscoveredSpecLinks(baseURL string, links []string, allowCrossOrigin bool) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	var out []string
	for _, raw := range links {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		if !allowCrossOrigin {
			if !strings.EqualFold(u.Hostname(), base.Hostname()) {
				continue
			}
		} else if isDisallowedCrossOriginHost(base.Hostname(), u.Hostname()) {
			continue
		}
		out = append(out, u.String())
	}
	return out
}

func isDisallowedCrossOriginHost(baseHost, host string) bool {
	basePrivate := hostIsNonPublic(baseHost)
	targetPrivate := hostIsNonPublic(host)
	return targetPrivate && !basePrivate
}

func hostIsNonPublic(host string) bool {
	host = strings.Trim(strings.TrimSuffix(host, "."), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return isNonPublicIP(ip)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if isNonPublicIP(addr.IP) {
			return true
		}
	}
	return false
}

func isNonPublicIP(ip net.IP) bool {
	return ip.IsPrivate() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
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

// loadSpecFiles loads the ordered list of spec files from cfg.SpecFiles,
// merges them in order (later entries win on conflict), and returns a single
// APISpec whose Raw bytes are re-serialized YAML.
func loadSpecFiles(ctx context.Context, cfg DiscoverConfig, loaders []Loader) (*APISpec, error) {
	tr := effectiveTransport(cfg)

	// Fast path: single file needs no merge; avoid an extra unmarshal+marshal.
	if len(cfg.SpecFiles) == 1 {
		src := cfg.SpecFiles[0]
		var ct string
		var data []byte
		var err error
		var opts LoadOptions
		if isLocalPath(src) {
			ct, data, err = readLocalFile(src)
			if localPath, pathErr := localPathFromSource(src); pathErr == nil {
				opts.LocalPath = localPath
			}
		} else {
			ct, data, _, err = fetchBytes(ctx, src, tr)
			opts.SourceURL = src
			opts.AllowCrossOrigin = cfg.AllowCrossOrigin
		}
		if err != nil {
			return nil, fmt.Errorf("spec file %q: %w", src, err)
		}
		opts.Context = ctx
		opts.Transport = tr
		return loadWithOptions(ct, data, loaders, opts)
	}

	var merged map[string]any
	var lastCT string

	for _, src := range cfg.SpecFiles {
		var ct string
		var data []byte
		var err error

		if isLocalPath(src) {
			ct, data, err = readLocalFile(src)
		} else {
			ct, data, _, err = fetchBytes(ctx, src, tr)
		}
		if err != nil {
			return nil, fmt.Errorf("spec file %q: %w", src, err)
		}

		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("spec file %q: parse: %w", src, err)
		}
		merged = deepMerge(merged, doc)
		lastCT = ct
	}

	if merged == nil {
		return nil, nil
	}

	// Re-serialize the merged document as YAML so existing loaders can parse it.
	raw, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("merging spec files: re-serialise: %w", err)
	}
	if lastCT == "" {
		lastCT = "application/yaml"
	}
	return load(lastCT, raw, loaders)
}

// isLocalPath reports whether s is a local filesystem path rather than a URL.
// A string is local if it has no scheme (no "://") or uses the "file://" scheme.
func isLocalPath(s string) bool {
	if strings.HasPrefix(s, "file://") {
		return true
	}
	return !strings.Contains(s, "://")
}

// readLocalFile reads a local spec file, stripping any leading "file://" prefix.
// The content-type is inferred from the file extension.
func readLocalFile(path string) (contentType string, data []byte, err error) {
	path, err = localPathFromSource(path)
	if err != nil {
		return "", nil, err
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		contentType = "application/json"
	default:
		contentType = "application/yaml"
	}
	return contentType, data, nil
}

func localPathFromSource(src string) (string, error) {
	if strings.HasPrefix(src, "file://") {
		u, err := url.Parse(src)
		if err != nil {
			return "", err
		}
		if u.Host != "" && u.Host != "localhost" {
			return "", fmt.Errorf("unsupported file URL host %q", u.Host)
		}
		return filepath.Clean(u.Path), nil
	}
	return filepath.Clean(src), nil
}

// deepMerge recursively merges overlay into base. overlay values take
// precedence; maps are merged recursively; all other types are replaced.
// Returns a new map; base and overlay are not modified.
func deepMerge(base, overlay map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		if bv, ok := result[k]; ok {
			if bMap, ok := bv.(map[string]any); ok {
				if oMap, ok := v.(map[string]any); ok {
					result[k] = deepMerge(bMap, oMap)
					continue
				}
			}
		}
		result[k] = v
	}
	return result
}
