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
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/rest-sh/restish/v2/internal/secrets"
	"go.yaml.in/yaml/v3"
)

// maxSpecBytes caps the body read during spec discovery (50 MiB).
// OpenAPI specs are rarely larger than a few megabytes; this prevents an
// untrusted server from exhausting memory during api connect / api sync.
const maxSpecBytes = 50 * 1024 * 1024

const defaultDiscoverTimeout = 30 * time.Second
const defaultExplicitSpecDiscoverTimeout = 2 * time.Minute

var ErrNoSpecFound = errors.New("no API spec found")

var errNoSpecCandidate = errors.New("no spec at candidate URL")

var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

type discoveryResult struct {
	spec      *APISpec
	ttl       time.Duration
	err       error
	priority  int // 0 = explicit SpecURL (preferred for errors); 1 = heuristic probes
	sourceURL string
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
	// Fetch, when set, is used for all HTTP fetches instead of Transport.
	Fetch HTTPFetcher
	// AllowCrossOrigin permits Link-header-discovered spec URLs on other hosts.
	// When false, only same-host discovered links are followed. Private/local
	// cross-origin targets are still rejected unless the base URL is already in
	// that trust class.
	AllowCrossOrigin bool
	// ForceRefresh bypasses any cached entry and rebuilds it from the source.
	ForceRefresh bool
	// Timeout is used when the caller context has no deadline. Zero selects a
	// default based on discovery mode.
	Timeout time.Duration
	// Trace receives verbose discovery progress messages.
	Trace func(format string, args ...any)
	// Warnf receives non-fatal operation extraction warnings.
	Warnf func(format string, args ...any)
}

// Discover returns the APISpec for an API, using cache when available.
// Discovery order (first success wins, network steps run in parallel):
//  1. CBOR spec cache
//  2. Explicit SpecURL (if configured)
//  3. Link headers from a GET on BaseURL (service-desc / service-doc / describedby)
//  4. Well-known paths /openapi.json and /openapi.yaml
//  5. BaseURL body itself
func Discover(ctx context.Context, cfg DiscoverConfig, loaders []Loader) (*APISpec, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, discoverTimeout(cfg))
		defer cancel()
	}
	tracef(cfg.Trace, "OpenAPI discovery for %q: base=%s spec=%s", cfg.APIName, cleanSourceURL(cfg.BaseURL), cleanSourceURL(cfg.SpecURL))

	// 1. Cache check (synchronous, no network).
	if cfg.CacheDir != "" && !cfg.ForceRefresh {
		if entry, ok := readCache(cfg.CacheDir, cfg.APIName, cfg.Version); ok {
			if !cacheSourceMatches(cfg, entry) {
				goto loadFresh
			}
			if specFilesChangedSince(cfg.SpecFiles, entry.FetchedAt) {
				goto loadFresh
			}
			opts := entry.loadOptions()
			opts.Context = ctx
			opts.Transport = effectiveTransport(cfg)
			opts.Fetch = effectiveFetcher(cfg)
			opts.Trace = cfg.Trace
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
			opts := OperationOptions{BaseURL: cfg.BaseURL, OperationBase: cfg.OperationBase, ServerVariables: cfg.ServerVariables, Warnf: cfg.Warnf}
			set, _ := spec.OperationSet(opts)
			entry := &cacheEntry{
				Version:   cfg.Version,
				FetchedAt: time.Now(),
				ExpiresAt: time.Now().Add(24 * time.Hour),
				Spec: cachedRaw{
					ContentType:      spec.ContentType,
					Raw:              spec.Raw,
					DiscoveryBaseURL: cleanSourceURL(cfg.BaseURL),
					SourceURL:        spec.SourceURL,
					LocalPath:        spec.LocalPath,
					AllowCrossOrigin: spec.AllowCrossOrigin,
				},
			}
			entry.SpecFiles = cacheSpecFileMetadata(cfg.SpecFiles)
			if set.Operations != nil {
				entry.upsertOperationSet(cacheOperationOptions(opts), set)
			}
			if err := writeCache(cfg.CacheDir, cfg.APIName, entry); err != nil {
				return nil, err
			}
		}
		return spec, nil
	}

	// 3-6. Network discovery (parallel).
	spec, ttl, err := discoverFromNetwork(ctx, cfg, loaders)
	if err != nil {
		return nil, err
	}
	if spec != nil && cfg.SpecURL != "" && spec.RequestedURL == "" {
		spec.RequestedURL = cleanSourceURL(cfg.SpecURL)
	}

	// Cache the result.
	if cfg.CacheDir != "" && spec != nil {
		var expiresAt time.Time
		if ttl > 0 {
			expiresAt = time.Now().Add(ttl)
		} else {
			expiresAt = time.Now().Add(24 * time.Hour)
		}
		opts := OperationOptions{BaseURL: cfg.BaseURL, OperationBase: cfg.OperationBase, ServerVariables: cfg.ServerVariables, Warnf: cfg.Warnf}
		set, _ := spec.OperationSet(opts)
		entry := &cacheEntry{
			Version:   cfg.Version,
			FetchedAt: time.Now(),
			ExpiresAt: expiresAt,
			Spec: cachedRaw{
				ContentType:      spec.ContentType,
				Raw:              spec.Raw,
				DiscoveryBaseURL: cleanSourceURL(cfg.BaseURL),
				RequestedURL:     cleanSourceURL(cfg.SpecURL),
				SourceURL:        spec.SourceURL,
				LocalPath:        spec.LocalPath,
				AllowCrossOrigin: spec.AllowCrossOrigin,
			},
		}
		if set.Operations != nil {
			entry.upsertOperationSet(cacheOperationOptions(opts), set)
		}
		if err := writeCache(cfg.CacheDir, cfg.APIName, entry); err != nil {
			return nil, err
		}
	}

	return spec, nil
}

func discoverTimeout(cfg DiscoverConfig) time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	if cfg.SpecURL != "" || len(cfg.SpecFiles) > 0 {
		return defaultExplicitSpecDiscoverTimeout
	}
	return defaultDiscoverTimeout
}

func tracef(trace func(format string, args ...any), format string, args ...any) {
	if trace != nil {
		trace(format, args...)
	}
}

func cacheSourceMatches(cfg DiscoverConfig, entry *cacheEntry) bool {
	if entry == nil {
		return false
	}
	entry.normalize()
	if entry.Spec.DiscoveryBaseURL != "" && !sourceURLMatches(entry.Spec.DiscoveryBaseURL, cfg.BaseURL) {
		return false
	}
	if len(cfg.SpecFiles) > 0 {
		return cacheSpecFilesMatch(cfg.SpecFiles, entry)
	}
	if cfg.SpecURL != "" {
		if entry.Spec.RequestedURL != "" {
			return sourceURLMatches(entry.Spec.RequestedURL, cfg.SpecURL)
		}
		return sourceURLMatches(entry.Spec.SourceURL, cfg.SpecURL)
	}
	return true
}

func sourceURLMatches(got, want string) bool {
	return cleanSourceURL(got) == cleanSourceURL(want)
}

func cacheSpecFilesMatch(specFiles []string, entry *cacheEntry) bool {
	if len(specFiles) == 0 {
		return true
	}
	if entry == nil {
		return false
	}
	entry.normalize()
	if len(entry.SpecFiles) > 0 {
		return cacheSpecFileMetadataMatches(specFiles, entry.SpecFiles)
	}
	if len(specFiles) != 1 {
		return false
	}
	src := specFiles[0]
	if isLocalPath(src) {
		path, err := localPathFromSource(src)
		if err != nil {
			return false
		}
		return entry.Spec.LocalPath == path
	}
	return sourceURLMatches(entry.Spec.SourceURL, src)
}

func cacheSpecFileMetadata(specFiles []string) []cachedSpecFile {
	if len(specFiles) == 0 {
		return nil
	}
	out := make([]cachedSpecFile, 0, len(specFiles))
	for _, src := range specFiles {
		meta := cachedSpecFile{Source: src}
		if isLocalPath(src) {
			meta.Local = true
			path, err := localPathFromSource(src)
			if err == nil {
				meta.Path = path
				if info, statErr := os.Stat(path); statErr == nil {
					meta.ModTime = info.ModTime()
					meta.ModTimeUnixNano = info.ModTime().UnixNano()
					meta.Size = info.Size()
				}
			}
		} else {
			meta.Source = cleanSourceURL(src)
		}
		out = append(out, meta)
	}
	return out
}

func cacheSpecFileMetadataMatches(specFiles []string, cached []cachedSpecFile) bool {
	current := cacheSpecFileMetadata(specFiles)
	if len(current) != len(cached) {
		return false
	}
	for i := range current {
		if current[i].Source != cached[i].Source ||
			current[i].Local != cached[i].Local ||
			current[i].Path != cached[i].Path ||
			current[i].Size != cached[i].Size {
			return false
		}
		if current[i].Local && !cachedSpecFileModTimeMatches(current[i], cached[i]) {
			return false
		}
	}
	return true
}

func cachedSpecFileModTimeMatches(current, cached cachedSpecFile) bool {
	if current.ModTimeUnixNano != 0 && cached.ModTimeUnixNano != 0 {
		return current.ModTimeUnixNano == cached.ModTimeUnixNano
	}
	if current.ModTime.Equal(cached.ModTime) {
		return true
	}
	// Legacy cache entries stored time.Time values that could round-trip
	// through CBOR at whole-second precision. Keep those caches usable when the
	// path, size, and second-level mtime still match; specFilesChangedSince
	// separately rejects local files modified after the cache was written.
	return current.ModTime.Truncate(time.Second).Equal(cached.ModTime.Truncate(time.Second))
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

	initialProbes := 1
	if cfg.SpecURL == "" {
		initialProbes = 1 + len(wellKnownSpecPaths)
	}
	ch := make(chan discoveryResult, initialProbes)
	var wg sync.WaitGroup
	tr := effectiveTransport(cfg)
	fetch := effectiveFetcher(cfg)

	launch := func(priority int, sourceURL string, fn func() (string, []byte, time.Duration, string, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct, body, ttl, effectiveSourceURL, err := fn()
			if effectiveSourceURL == "" {
				effectiveSourceURL = sourceURL
			}
			if err != nil {
				if priority > 0 && errors.Is(err, errNoSpecCandidate) {
					return
				}
				if ctx.Err() != nil {
					select {
					case ch <- discoveryResult{err: err, priority: priority, sourceURL: effectiveSourceURL}:
					default:
					}
					return
				}
				select {
				case ch <- discoveryResult{err: err, priority: priority, sourceURL: effectiveSourceURL}:
				case <-ctx.Done():
				}
				return
			}
			spec, loadErr := loadWithOptions(ct, body, loaders, LoadOptions{
				Context:          ctx,
				SourceURL:        effectiveSourceURL,
				AllowCrossOrigin: cfg.AllowCrossOrigin,
				Transport:        tr,
				Fetch:            fetch,
				Trace:            cfg.Trace,
			})
			if spec != nil && spec.SourceURL == "" {
				spec.SourceURL = effectiveSourceURL
			}
			if priority == 0 && spec == nil && loadErr == nil {
				loadErr = fmt.Errorf("GET %s: unsupported API spec: expected an OpenAPI 3.x document", effectiveSourceURL)
			}
			if loadErr != nil && ctx.Err() != nil {
				select {
				case ch <- discoveryResult{spec: spec, ttl: ttl, err: loadErr, priority: priority, sourceURL: effectiveSourceURL}:
				default:
				}
				return
			}
			select {
			case ch <- discoveryResult{spec: spec, ttl: ttl, err: loadErr, priority: priority, sourceURL: effectiveSourceURL}:
			case <-ctx.Done():
			}
		}()
	}

	// Explicit spec URL (priority 0 — most authoritative error source).
	if cfg.SpecURL != "" {
		u := cfg.SpecURL
		launch(0, u, func() (string, []byte, time.Duration, string, error) {
			ct, body, ttl, sourceURL, err := fetchBytes(ctx, u, tr, fetch, cfg.Trace)
			if errors.Is(err, errNoSpecCandidate) {
				return "", nil, 0, sourceURL, fmt.Errorf("GET %s: 404 Not Found", sourceURL)
			}
			return ct, body, ttl, sourceURL, err
		})
		go func() {
			wg.Wait()
			close(ch)
		}()
		spec, ttl, err := collectDiscoveryResults(ctx, cancel, ch, cfg.BaseURL)
		cancel()
		wg.Wait()
		return spec, ttl, err
	}

	// Probe base URL: extract Link headers and try the body itself.
	baseURL := cfg.BaseURL
	launch(1, baseURL, func() (string, []byte, time.Duration, string, error) {
		ct, body, ttl, sourceURL, linkURLs, err := fetchWithLinks(ctx, baseURL, tr, fetch, cfg.AllowCrossOrigin, cfg.Trace)
		if err != nil {
			return "", nil, 0, sourceURL, err
		}
		// Launch Link-header candidates as additional probes.
		// Calling launch() from within a goroutine tracked by wg is safe:
		// this goroutine's wg.Done() hasn't been deferred yet, so the
		// counter is still ≥1 when the inner wg.Add(1) is called.
		for _, lu := range linkURLs {
			u := lu
			launch(1, u, func() (string, []byte, time.Duration, string, error) {
				return fetchBytes(ctx, u, tr, fetch, cfg.Trace)
			})
		}
		return ct, body, ttl, sourceURL, nil
	})

	// Well-known paths.
	for _, path := range wellKnownSpecPaths {
		u := joinURL(cfg.BaseURL, path)
		launch(1, u, func() (string, []byte, time.Duration, string, error) {
			return fetchBytes(ctx, u, tr, fetch, cfg.Trace)
		})
	}

	// Close ch once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	spec, ttl, err := collectDiscoveryResults(ctx, cancel, ch, cfg.BaseURL)
	cancel()
	wg.Wait()
	return spec, ttl, err
}

var wellKnownSpecPaths = []string{"/openapi.json", "/openapi.yaml"}

func collectDiscoveryResults(ctx context.Context, cancel context.CancelFunc, ch <-chan discoveryResult, baseURL string) (*APISpec, time.Duration, error) {
	// Collect errors, preferring lower-priority values (0 = SpecURL is most
	// authoritative). Same-priority errors are joined so all causes are visible.
	bestErrPriority := math.MaxInt
	var bestErrs []error
	for r := range ch {
		if r.spec != nil {
			if r.spec.SourceURL == "" {
				r.spec.SourceURL = r.sourceURL
			}
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
	return nil, 0, fmt.Errorf("%w at %s", ErrNoSpecFound, cleanSourceURL(baseURL))
}

// fetchBytes performs a GET and returns content-type, body, cache TTL, effective source URL, and error.
func fetchBytes(ctx context.Context, rawURL string, tr http.RoundTripper, fetch HTTPFetcher, trace func(format string, args ...any)) (string, []byte, time.Duration, string, error) {
	ct, body, ttl, sourceURL, _, err := fetchWithLinks(ctx, rawURL, tr, fetch, true, trace)
	return ct, body, ttl, sourceURL, err
}

// fetchWithLinks performs a GET and also returns parsed Link header spec URLs.
func fetchWithLinks(ctx context.Context, rawURL string, tr http.RoundTripper, fetch HTTPFetcher, allowCrossOrigin bool, trace func(format string, args ...any)) (ct string, body []byte, ttl time.Duration, sourceURL string, links []string, err error) {
	displayURL := cleanSourceURL(rawURL)
	tracef(trace, "GET OpenAPI source %s", displayURL)
	resp, err := fetch(ctx, rawURL, tr)
	if err != nil {
		return "", nil, 0, displayURL, nil, fmt.Errorf("GET %s: %w", displayURL, cleanErrorForDisplay(err, rawURL, displayURL))
	}
	if resp == nil {
		return "", nil, 0, displayURL, nil, fmt.Errorf("GET %s: no response", displayURL)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	sourceURL = effectiveResponseSourceURL(rawURL, resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound {
			return "", nil, 0, sourceURL, nil, errNoSpecCandidate
		}
		return "", nil, 0, sourceURL, nil, fmt.Errorf("GET %s: %s", sourceURL, resp.Status)
	}

	if resp.Body != nil {
		body, err = io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes+1))
	}
	if err != nil {
		return "", nil, 0, sourceURL, nil, err
	}
	if int64(len(body)) > maxSpecBytes {
		return "", nil, 0, sourceURL, nil, fmt.Errorf("spec body from %s exceeds limit of %d bytes", sourceURL, maxSpecBytes)
	}

	ct = resp.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}

	ttl = cacheTTL(resp)
	links = filterDiscoveredSpecLinks(sourceURL, extractSpecLinks(sourceURL, resp.Header), allowCrossOrigin)
	return ct, body, ttl, sourceURL, links, nil
}

func effectiveResponseSourceURL(rawURL string, resp *http.Response) string {
	normalizedRaw := rawURL
	if normalized, err := request.Normalize(rawURL, ""); err == nil {
		normalizedRaw = normalized
	}
	base, baseErr := url.Parse(normalizedRaw)
	if resp == nil || resp.Request == nil || resp.Request.URL == nil || baseErr != nil {
		return cleanSourceURL(rawURL)
	}
	source := *resp.Request.URL
	source.User = nil
	source.Fragment = ""
	if resp.Request.Response != nil {
		source.RawQuery = cleanSourceURLQuery(source.Query()).Encode()
	} else {
		source.RawQuery = cleanSourceURLQuery(base.Query()).Encode()
	}
	return source.String()
}

func cleanSourceURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	cleaned := raw
	if normalized, err := request.Normalize(raw, ""); err == nil {
		cleaned = normalized
	}
	u, err := url.Parse(cleaned)
	if err != nil || !u.IsAbs() {
		return cleanPossiblyInvalidURL(cleaned)
	}
	u.User = nil
	u.Fragment = ""
	u.RawQuery = cleanSourceURLQuery(u.Query()).Encode()
	return u.String()
}

func cleanSourceURLQuery(values url.Values) url.Values {
	cleaned := url.Values{}
	for name, vals := range values {
		if request.IsCredentialQueryParam(name) {
			continue
		}
		for _, value := range vals {
			if secrets.IsQueryParamValue(name, value) {
				continue
			}
			cleaned.Add(name, value)
		}
	}
	return cleaned
}

func cleanPossiblyInvalidURL(raw string) string {
	return secrets.RedactDiagnosticURLText(raw)
}

type displayError struct {
	err     error
	raw     string
	display string
}

func (e displayError) Error() string {
	msg := e.err.Error()
	if e.raw != "" && e.display != "" {
		msg = strings.ReplaceAll(msg, e.raw, e.display)
	}
	return cleanPossiblyInvalidURL(msg)
}

func (e displayError) Unwrap() error {
	return e.err
}

func cleanErrorForDisplay(err error, raw, display string) error {
	if err == nil {
		return nil
	}
	return displayError{err: err, raw: raw, display: display}
}

// effectiveTransport returns cfg.Transport when set, or http.DefaultTransport.
func effectiveTransport(cfg DiscoverConfig) http.RoundTripper {
	if cfg.Transport != nil {
		return cfg.Transport
	}
	return http.DefaultTransport
}

func effectiveFetcher(cfg DiscoverConfig) HTTPFetcher {
	if cfg.Fetch != nil {
		return cfg.Fetch
	}
	return func(ctx context.Context, rawURL string, tr http.RoundTripper) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		return tr.RoundTrip(req)
	}
}

// cacheTTL extracts the cache duration from a response's Cache-Control header.
func cacheTTL(resp *http.Response) time.Duration {
	cc := resp.Header.Get("Cache-Control")
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(part)
		if strings.EqualFold(part, "no-store") {
			return 0
		}
	}
	for _, directive := range []string{"s-maxage=", "max-age="} {
		for _, part := range strings.Split(cc, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), directive) {
				if secs, err := strconv.Atoi(part[len(directive):]); err == nil && secs > 0 {
					return time.Duration(secs) * time.Second
				}
				return 0
			}
		}
	}
	return 0
}

// extractSpecLinks parses Link response headers and returns URLs whose rel is
// "service-desc", "service-doc", or "describedby".
func extractSpecLinks(baseURL string, h http.Header) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	var out []string
	for _, parsed := range hypermedia.LinkHeaderLinks(base, h) {
		if isSpecLinkRel(parsed.Rel) {
			out = append(out, parsed.URI)
		}
	}
	return out
}

func isSpecLinkRel(rel string) bool {
	switch strings.ToLower(rel) {
	case "service-desc", "service-doc", "describedby":
		return true
	default:
		return false
	}
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
			if !sameOrigin(base, u) {
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
	basePrivate, baseOK := hostNonPublicStatus(baseHost)
	targetPrivate, targetOK := hostNonPublicStatus(host)
	if !targetOK {
		return true
	}
	return targetPrivate && !(baseOK && basePrivate)
}

func hostIsNonPublic(host string) bool {
	nonPublic, ok := hostNonPublicStatus(host)
	return nonPublic || !ok
}

func hostNonPublicStatus(host string) (nonPublic bool, ok bool) {
	host = strings.Trim(strings.TrimSuffix(host, "."), "[]")
	if strings.EqualFold(host, "localhost") {
		return true, true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return isNonPublicIP(ip), true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return true, false
	}
	for _, addr := range addrs {
		if isNonPublicIP(addr.IP) {
			return true, true
		}
	}
	return false, true
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
// APISpec whose Raw bytes are re-serialized YAML. Multi-file merging parses
// documents into generic maps, so YAML anchors, aliases, comments, and exact
// scalar spellings are not preserved in the merged representation.
func loadSpecFiles(ctx context.Context, cfg DiscoverConfig, loaders []Loader) (*APISpec, error) {
	tr := effectiveTransport(cfg)
	fetch := effectiveFetcher(cfg)

	// Fast path: single file needs no merge; avoid an extra unmarshal+marshal.
	if len(cfg.SpecFiles) == 1 {
		src := cfg.SpecFiles[0]
		displaySrc := specFileDisplaySource(src)
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
			var sourceURL string
			ct, data, _, sourceURL, err = fetchBytes(ctx, src, tr, fetch, cfg.Trace)
			opts.SourceURL = sourceURL
			opts.AllowCrossOrigin = cfg.AllowCrossOrigin
		}
		if err != nil {
			return nil, fmt.Errorf("spec file %q: %w", displaySrc, err)
		}
		opts.Context = ctx
		opts.Transport = tr
		opts.Fetch = fetch
		opts.Trace = cfg.Trace
		spec, err := loadWithOptions(ct, data, loaders, opts)
		if err != nil {
			return nil, err
		}
		if spec == nil {
			return nil, fmt.Errorf("spec file %q: unsupported API spec: expected an OpenAPI 3.x document", displaySrc)
		}
		return spec, nil
	}

	var merged map[string]any
	var lastCT string

	for _, src := range cfg.SpecFiles {
		displaySrc := specFileDisplaySource(src)
		var ct string
		var data []byte
		var err error
		opts := LoadOptions{Context: ctx, Transport: tr, Fetch: fetch, Trace: cfg.Trace}

		if isLocalPath(src) {
			ct, data, err = readLocalFile(src)
			if localPath, pathErr := localPathFromSource(src); pathErr == nil {
				opts.LocalPath = localPath
			}
		} else {
			var sourceURL string
			ct, data, _, sourceURL, err = fetchBytes(ctx, src, tr, fetch, cfg.Trace)
			opts.SourceURL = sourceURL
			opts.AllowCrossOrigin = cfg.AllowCrossOrigin
		}
		if err != nil {
			return nil, fmt.Errorf("spec file %q: %w", displaySrc, err)
		}
		data, err = resolveOpenAPIExternalRefs(data, opts)
		if err != nil {
			return nil, fmt.Errorf("spec file %q: %w", displaySrc, err)
		}

		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("spec file %q: parse: %w", displaySrc, err)
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
	spec, err := load(lastCT, raw, loaders)
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, fmt.Errorf("spec files: unsupported API spec: expected an OpenAPI 3.x document")
	}
	return spec, nil
}

func specFileDisplaySource(src string) string {
	if isLocalPath(src) {
		return src
	}
	return cleanSourceURL(src)
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
