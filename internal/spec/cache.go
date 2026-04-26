package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// cacheEntry is the on-disk format for a cached spec.
type cacheEntry struct {
	Version    string    `cbor:"version"`
	FetchedAt  time.Time `cbor:"fetched_at"`
	ExpiresAt  time.Time `cbor:"expires_at"`
	Schema     int       `cbor:"schema,omitempty"`
	Spec       cachedRaw `cbor:"spec,omitempty"`
	Operations []opsBlob `cbor:"operations,omitempty"`

	// Legacy v2-dev fields. Keep them readable so older raw-only cache entries
	// can be upgraded opportunistically after a successful parse.
	ContentType string `cbor:"content_type,omitempty"`
	Raw         []byte `cbor:"raw,omitempty"`
}

type cachedRaw struct {
	ContentType string `cbor:"content_type,omitempty"`
	Raw         []byte `cbor:"raw,omitempty"`
}

type opsBlob struct {
	Schema             int         `cbor:"schema"`
	BaseURL            string      `cbor:"base_url"`
	OperationBase      string      `cbor:"operation_base,omitempty"`
	ServerVariablesKey string      `cbor:"server_variables,omitempty"`
	RawSHA256          string      `cbor:"raw_sha256"`
	Info               APIInfo     `cbor:"info,omitempty"`
	Operations         []Operation `cbor:"operations"`
}

const currentCacheSchema = 2
const currentOperationCacheSchema = 4

// cacheFile returns the path of the CBOR cache file for the given API.
func cacheFile(cacheDir, apiName string) string {
	return filepath.Join(cacheDir, apiName+".cbor")
}

func validCacheAPIName(apiName string) bool {
	if apiName == "" || apiName == "." || apiName == ".." {
		return false
	}
	if strings.ContainsAny(apiName, `/\`) {
		return false
	}
	return filepath.Base(apiName) == apiName
}

// readCache loads and validates a cached spec entry.
// Returns the entry and true if the cache is valid (not expired, version matches).
func readCache(cacheDir, apiName, version string) (*cacheEntry, bool) {
	if !validCacheAPIName(apiName) {
		return nil, false
	}
	path := cacheFile(cacheDir, apiName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var e cacheEntry
	if err := cbor.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	if e.Version != version {
		return nil, false // restish version changed
	}
	if !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt) {
		return nil, false // TTL expired
	}
	e.normalize()
	return &e, true
}

// writeCache serialises entry to the CBOR cache file.
func writeCache(cacheDir, apiName string, entry *cacheEntry) error {
	if !validCacheAPIName(apiName) {
		return fmt.Errorf("spec cache: invalid API name %q", apiName)
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return fmt.Errorf("spec cache: mkdir: %w", err)
	}
	entry.normalize()
	if entry.Schema == 0 {
		entry.Schema = currentCacheSchema
	}
	data, err := cbor.Marshal(entry)
	if err != nil {
		return fmt.Errorf("spec cache: marshal: %w", err)
	}
	return os.WriteFile(cacheFile(cacheDir, apiName), data, 0o600)
}

func (e *cacheEntry) normalize() {
	if e == nil {
		return
	}
	if len(e.Spec.Raw) == 0 && len(e.Raw) > 0 {
		e.Spec.Raw = e.Raw
	}
	if e.Spec.ContentType == "" && e.ContentType != "" {
		e.Spec.ContentType = e.ContentType
	}
}

func (e *cacheEntry) contentType() string {
	if e == nil {
		return ""
	}
	e.normalize()
	return e.Spec.ContentType
}

func (e *cacheEntry) raw() []byte {
	if e == nil {
		return nil
	}
	e.normalize()
	return e.Spec.Raw
}

// LoadFromCache reads the cached spec for apiName, re-parses it using loaders,
// and returns the result. Returns nil, nil when the cache is empty or expired.
func LoadFromCache(cacheDir, apiName, version string, specFiles []string, loaders []Loader) (*APISpec, error) {
	entry, ok := readCache(cacheDir, apiName, version)
	if !ok {
		return nil, nil
	}
	if specFilesChangedSince(specFiles, entry.FetchedAt) {
		return nil, nil
	}
	return load(entry.contentType(), entry.raw(), loaders)
}

// LoadOperationsFromCache reads extracted operations for a cached spec without
// re-parsing the raw OpenAPI document. The bool return is false when the cache
// entry is missing, expired, stale against local files, or lacks operations for
// the requested base URL and operation base.
func LoadOperationsFromCache(cacheDir, apiName, version string, specFiles []string, baseURL, operationBase string) ([]Operation, bool) {
	set, ok := LoadOperationSetFromCache(cacheDir, apiName, version, specFiles, baseURL, operationBase)
	return set.Operations, ok
}

// LoadOperationSetFromCache reads extracted operations and API metadata for a
// cached spec without reparsing the raw OpenAPI document.
func LoadOperationSetFromCache(cacheDir, apiName, version string, specFiles []string, baseURL, operationBase string) (OperationSet, bool) {
	return LoadOperationSetFromCacheWithVariables(cacheDir, apiName, version, specFiles, OperationOptions{
		BaseURL:       baseURL,
		OperationBase: operationBase,
	})
}

// LoadOperationSetFromCacheWithVariables reads extracted operations and API
// metadata for a cache key that includes OpenAPI server variable values.
func LoadOperationSetFromCacheWithVariables(cacheDir, apiName, version string, specFiles []string, opts OperationOptions) (OperationSet, bool) {
	entry, ok := readCache(cacheDir, apiName, version)
	if !ok {
		return OperationSet{}, false
	}
	if specFilesChangedSince(specFiles, entry.FetchedAt) {
		return OperationSet{}, false
	}
	rawHash := cacheRawHash(entry.raw())
	for _, blob := range entry.Operations {
		if blob.Schema != currentOperationCacheSchema {
			continue
		}
		if blob.BaseURL == opts.BaseURL &&
			blob.OperationBase == opts.OperationBase &&
			blob.ServerVariablesKey == ServerVariablesCacheKey(opts.ServerVariables) &&
			blob.RawSHA256 == rawHash {
			set := OperationSet{
				Info:       blob.Info,
				Operations: append([]Operation(nil), blob.Operations...),
			}
			if blob.Operations == nil {
				set.Operations = []Operation{}
			}
			return set, true
		}
	}
	return OperationSet{}, false
}

// StoreOperationsInCache updates an existing raw cache entry with extracted
// operations. It is best-effort for callers; failed upgrades should not make
// startup fail when the raw cache can still be parsed.
func StoreOperationsInCache(cacheDir, apiName, version, baseURL, operationBase string, ops []Operation) error {
	return StoreOperationSetInCache(cacheDir, apiName, version, baseURL, operationBase, OperationSet{Operations: ops})
}

// StoreOperationSetInCache updates an existing raw cache entry with extracted
// operations and API metadata.
func StoreOperationSetInCache(cacheDir, apiName, version, baseURL, operationBase string, set OperationSet) error {
	return StoreOperationSetInCacheWithVariables(cacheDir, apiName, version, OperationOptions{
		BaseURL:       baseURL,
		OperationBase: operationBase,
	}, set)
}

// StoreOperationSetInCacheWithVariables updates an existing raw cache entry
// with operation metadata keyed by server variable values.
func StoreOperationSetInCacheWithVariables(cacheDir, apiName, version string, opts OperationOptions, set OperationSet) error {
	entry, ok := readCache(cacheDir, apiName, version)
	if !ok {
		return nil
	}
	entry.upsertOperationSetWithOptions(opts, set)
	return writeCache(cacheDir, apiName, entry)
}

func (e *cacheEntry) upsertOperations(baseURL, operationBase string, ops []Operation) {
	e.upsertOperationSet(baseURL, operationBase, OperationSet{Operations: ops})
}

func (e *cacheEntry) upsertOperationSet(baseURL, operationBase string, set OperationSet) {
	e.upsertOperationSetWithOptions(OperationOptions{BaseURL: baseURL, OperationBase: operationBase}, set)
}

func (e *cacheEntry) upsertOperationSetWithOptions(opts OperationOptions, set OperationSet) {
	rawHash := cacheRawHash(e.raw())
	blob := opsBlob{
		Schema:             currentOperationCacheSchema,
		BaseURL:            opts.BaseURL,
		OperationBase:      opts.OperationBase,
		ServerVariablesKey: ServerVariablesCacheKey(opts.ServerVariables),
		RawSHA256:          rawHash,
		Info:               set.Info,
		Operations:         append([]Operation(nil), set.Operations...),
	}
	for i := range e.Operations {
		if e.Operations[i].BaseURL == opts.BaseURL &&
			e.Operations[i].OperationBase == opts.OperationBase &&
			e.Operations[i].ServerVariablesKey == blob.ServerVariablesKey {
			e.Operations[i] = blob
			return
		}
	}
	e.Operations = append(e.Operations, blob)
}

func cacheRawHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func HasLocalSpecFiles(specFiles []string) bool {
	for _, src := range specFiles {
		if isLocalPath(src) {
			return true
		}
	}
	return false
}

// InvalidateCache removes the cached spec file for apiName.
// Returns nil if the file did not exist.
func InvalidateCache(cacheDir, apiName string) error {
	if !validCacheAPIName(apiName) {
		return fmt.Errorf("spec cache: invalid API name %q", apiName)
	}
	err := os.Remove(cacheFile(cacheDir, apiName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
