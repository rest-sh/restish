package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/danielgtaylor/mexpr"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/rest-sh/restish/v2/internal/fileutil"
	"github.com/rest-sh/restish/v2/internal/request"
)

const (
	metaDir     = ".rshbulk"
	metaFile    = ".rshbulk/meta"
	defaultJobs = 4
)

var (
	renameBulkFile = os.Rename
	removeBulkFile = os.Remove
)

type File struct {
	Path          string `json:"path"`
	URL           string `json:"url"`
	ETag          string `json:"etag,omitempty"`
	LastModified  string `json:"last_modified,omitempty"`
	VersionRemote string `json:"version_remote,omitempty"`
	VersionLocal  string `json:"version_local,omitempty"`
	Hash          []byte `json:"hash,omitempty"`
}

type Meta struct {
	URL         string           `json:"url"`
	Filter      string           `json:"filter,omitempty"`
	Base        string           `json:"base,omitempty"`
	URLTemplate string           `json:"url_template,omitempty"`
	Files       map[string]*File `json:"files,omitempty"`
}

type listEntry struct {
	URL     string
	Version string
}

func loadMeta() (*Meta, error) {
	data, err := os.ReadFile(metaFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("bulk checkout not initialized; run \"restish bulk init URL\" first")
		}
		return nil, err
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	if meta.Files == nil {
		meta.Files = map[string]*File{}
	}
	return &meta, nil
}

func (m *Meta) save() error {
	data, err := prettyJSON(m)
	if err != nil {
		return err
	}
	return atomicWriteBulkFile(metaFile, append(data, '\n'), 0o600)
}

func collectFiles(meta *Meta, args []string, match string, includeDeleted bool) ([]string, error) {
	if len(args) == 0 {
		seen := map[string]bool{}
		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path == metaDir {
					return filepath.SkipDir
				}
				return nil
			}
			rel := filepath.ToSlash(path)
			if rel == metaDir || strings.HasPrefix(rel, metaDir+"/") {
				return nil
			}
			args = append(args, rel)
			seen[rel] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
		if includeDeleted {
			for _, f := range meta.Files {
				if !seen[f.Path] {
					args = append(args, f.Path)
				}
			}
		}
	}

	if match != "" {
		ast, err := mexpr.Parse(match, nil, mexpr.UnquotedStrings)
		if err != nil {
			return nil, err
		}
		interpreter := mexpr.NewInterpreter(ast, mexpr.UnquotedStrings)
		filtered := make([]string, 0, len(args))
		for _, path := range args {
			data, err := os.ReadFile(path)
			if err != nil {
				if includeDeleted && errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, err
			}
			var content any
			if err := json.Unmarshal(data, &content); err != nil {
				continue
			}
			if err := mexpr.TypeCheck(ast, content, mexpr.UnquotedStrings); err != nil {
				continue
			}
			result, err := interpreter.Run(content)
			if err != nil || isFalsey(result) {
				continue
			}
			filtered = append(filtered, path)
		}
		args = filtered
	}

	sort.Strings(args)
	return args, nil
}

func (f *File) writeCached(body []byte) error {
	path := filepath.Join(metaDir, filepath.FromSlash(f.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o600)
}

func (f *File) write(body []byte) error {
	formatted, err := reformat(body)
	if err != nil {
		return err
	}
	f.Hash = hashBytes(formatted)
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(f.Path, append(formatted, '\n'), 0o600)
}

func (f *File) reset() error {
	data, err := os.ReadFile(filepath.Join(metaDir, filepath.FromSlash(f.Path)))
	if err != nil {
		return err
	}
	return f.write(data)
}

func (f *File) isChangedLocal(ignoreDeleted bool) (bool, error) {
	if len(f.Hash) == 0 {
		return false, nil
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return !ignoreDeleted, nil
	}
	formatted, err := reformat(data)
	if err != nil {
		return true, fmt.Errorf("%s contains invalid JSON: %w", f.Path, err)
	}
	return !bytes.Equal(hashBytes(formatted), f.Hash), nil
}

func (c changedFile) String() string {
	label := map[fileStatus]string{
		statusAdded:    "added",
		statusModified: "modified",
		statusRemoved:  "removed",
	}[c.Status]
	return fmt.Sprintf("\t%8s:  %s", label, c.File.Path)
}

func reformat(data []byte) ([]byte, error) {
	value, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	return prettyJSON(value)
}

func decodeJSON(data []byte) (any, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func prettyJSON(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func atomicWriteBulkFile(path string, data []byte, mode os.FileMode) error {
	return fileutil.AtomicWriteFile(path, data, fileutil.AtomicWriteOptions{
		FileMode:    mode,
		DirMode:     0o700,
		TempPattern: "." + filepath.Base(path) + "-*.tmp",
		Rename:      renameBulkFile,
		SyncDir:     true,
	})
}

func hashBytes(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func unifiedDiff(originalPath, modifiedPath string, original, modified []byte) string {
	original = normalizeDiffJSON(original)
	modified = normalizeDiffJSON(modified)
	edits := myers.ComputeEdits(span.URIFromPath(originalPath), string(original), string(modified))
	if len(edits) == 0 {
		return "No changes made.\n"
	}
	return fmt.Sprint(gotextdiff.ToUnified(originalPath, modifiedPath, string(original), edits))
}

func normalizeDiffJSON(data []byte) []byte {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	formatted, err := reformat(data)
	if err != nil {
		return bytes.TrimSpace(data)
	}
	return append(formatted, '\n')
}

func commonPrefix(base *url.URL, entries []listEntry) string {
	if len(entries) == 0 {
		return ""
	}
	resolved := make([]string, 0, len(entries))
	for _, entry := range entries {
		u, err := url.Parse(entry.URL)
		if err != nil {
			continue
		}
		resolved = append(resolved, base.ResolveReference(u).String())
	}
	if len(resolved) == 0 {
		return base.String()
	}
	if len(resolved) == 1 {
		return parentURLPrefix(resolved[0])
	}
	prefix := strings.Split(resolved[0], "/")
	for _, entry := range resolved[1:] {
		parts := strings.Split(entry, "/")
		for i, part := range parts {
			if len(prefix) == i || prefix[i] != part {
				prefix = prefix[:i]
				break
			}
		}
	}
	joined := strings.Join(prefix, "/")
	if joined != "" && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined
}

func parentURLPrefix(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	u.Fragment = ""
	escaped := u.EscapedPath()
	if escaped == "" {
		escaped = "/"
	}
	escaped = strings.TrimSuffix(escaped, "/")
	if escaped == "" {
		escaped = "/"
	}
	parent := path.Dir(escaped)
	if parent == "." {
		parent = "/"
	}
	if !strings.HasSuffix(parent, "/") {
		parent += "/"
	}
	unescaped, err := url.PathUnescape(parent)
	if err != nil {
		unescaped = parent
	}
	u.Path = unescaped
	u.RawPath = parent
	return u.String()
}

func getFirstKey(item any, keys ...string) string {
	m, ok := item.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range keys {
		if value, ok := m[key]; ok && value != nil {
			return fmt.Sprintf("%v", value)
		}
	}
	return ""
}

// urlTemplatePlaceholder matches {key} placeholders in URL templates.
var urlTemplatePlaceholder = regexp.MustCompile(`\{[^}]+\}`)

func renderURLTemplate(template string, item any) string {
	m, ok := item.(map[string]any)
	if !ok {
		return ""
	}
	return urlTemplatePlaceholder.ReplaceAllStringFunc(template, func(match string) string {
		key := strings.Trim(match, "{}")
		return url.PathEscape(fmt.Sprintf("%v", m[key]))
	})
}

func normalizedBaseURL(raw string) string {
	normalized, err := request.Normalize(raw, "")
	if err != nil {
		return raw
	}
	return normalized
}

func bulkRelativePath(baseURL, resolvedURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid bulk base URL %q: %w", baseURL, err)
	}
	resolved, err := url.Parse(resolvedURL)
	if err != nil {
		return "", fmt.Errorf("invalid bulk item URL %q: %w", resolvedURL, err)
	}
	if !strings.EqualFold(base.Scheme, resolved.Scheme) || !strings.EqualFold(base.Host, resolved.Host) {
		return "", fmt.Errorf("bulk item %q is outside checkout base %q", resolvedURL, baseURL)
	}

	basePath := path.Clean("/" + strings.TrimPrefix(base.EscapedPath(), "/"))
	resolvedPath := path.Clean("/" + strings.TrimPrefix(resolved.EscapedPath(), "/"))
	basePrefix := strings.TrimSuffix(basePath, "/") + "/"
	if basePath == "/" {
		basePrefix = "/"
	}
	if resolvedPath != strings.TrimSuffix(basePrefix, "/") && !strings.HasPrefix(resolvedPath, basePrefix) {
		return "", fmt.Errorf("bulk item %q escapes checkout base %q", resolvedURL, baseURL)
	}

	rel := strings.TrimPrefix(resolvedPath, basePrefix)
	rel = strings.TrimPrefix(rel, "/")
	cleaned := path.Clean(rel)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("bulk item %q resolves to invalid path %q", resolvedURL, rel)
	}
	return filepath.ToSlash(cleaned) + ".json", nil
}

func isFalsey(v any) bool {
	switch value := v.(type) {
	case nil:
		return true
	case bool:
		return !value
	case string:
		return value == ""
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	case float64:
		return value == 0
	case int:
		return value == 0
	default:
		return false
	}
}
