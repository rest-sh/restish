package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/internal/output"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
	"github.com/zeebo/xxh3"
)

func TestBulkRelativePath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		resolved string
		want     string
		wantErr  bool
	}{
		{
			name:     "valid child path",
			base:     "https://api.example.com/users/",
			resolved: "https://api.example.com/users/a/items/a1",
			want:     "a/items/a1.json",
		},
		{
			name:     "reject different host",
			base:     "https://api.example.com/users/",
			resolved: "https://attacker.example.com/users/a/items/a1",
			wantErr:  true,
		},
		{
			name:     "reject parent escape",
			base:     "https://api.example.com/users/a/",
			resolved: "https://api.example.com/users/secrets",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bulkRelativePath(tc.base, tc.resolved)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("bulkRelativePath: %v", err)
			}
			if got != tc.want {
				t.Fatalf("bulkRelativePath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHashBytesUsesV1CompatibleXXH3(t *testing.T) {
	data := []byte("{\n  \"id\": \"one\"\n}")
	got := hashBytes(data)
	want := xxh3.Hash128(data).Bytes()
	if len(got) != 16 {
		t.Fatalf("hash length = %d, want v1-compatible 128-bit hash", len(got))
	}
	if !bytes.Equal(got, want[:]) {
		t.Fatalf("hashBytes did not match xxh3.Hash128")
	}
}

func TestChangedFileStringColorizesStatusLabel(t *testing.T) {
	if err := output.SetTheme(output.ThemeEntries{"inserted": "#010203"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.SetTheme(nil) })

	changed := changedFile{Status: statusAdded, File: &File{Path: "one.json"}}
	got := changed.StringColor(true)
	if !strings.Contains(got, "\x1b[38;2;") || !strings.Contains(got, "added") {
		t.Fatalf("colored status = %q, want ANSI colored added label", got)
	}
	if !strings.Contains(got, "1;2;3") {
		t.Fatalf("colored status = %q, want configured theme color", got)
	}
	if plain := changed.String(); strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain status contains ANSI: %q", plain)
	}
}

func TestColorizeDiffUsesTerminalContext(t *testing.T) {
	a := &app{client: &pluginClient{term: pluginwire.TerminalContext{Color: true}}}
	got := a.colorizeDiff("--- old\n+++ new\n@@ -1 +1 @@\n-old\n+new\n")
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("colorized diff missing ANSI: %q", got)
	}
	a.client.term.Color = false
	plainInput := "--- old\n+++ new\n@@ -1 +1 @@\n-old\n+new\n"
	if plain := a.colorizeDiff(plainInput); plain != plainInput {
		t.Fatalf("color disabled changed diff: %q", plain)
	}
}

func TestCommonPrefixResolvesAgainstBaseURL(t *testing.T) {
	base, err := url.Parse("https://api.example.com/root/")
	if err != nil {
		t.Fatal(err)
	}
	got := commonPrefix(base, []listEntry{
		{URL: "https://api.example.com/root/users/a"},
		{URL: "/root/users/b"},
	})
	want := "https://api.example.com/root/users/"
	if got != want {
		t.Fatalf("commonPrefix = %q, want %q", got, want)
	}
}

func TestCommonPrefixSingleEntryUsesParentPath(t *testing.T) {
	base, err := url.Parse("https://api.example.com/root/index")
	if err != nil {
		t.Fatal(err)
	}
	got := commonPrefix(base, []listEntry{{URL: "/root/items/two"}})
	want := "https://api.example.com/root/items/"
	if got != want {
		t.Fatalf("commonPrefix = %q, want %q", got, want)
	}
}

func TestPullIndexRejectsMalformedEntryURL(t *testing.T) {
	var out bytes.Buffer
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &out),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			return &httpResponse{
				Status: 200,
				Body: []any{
					map[string]any{"url": "http://[::1", "version": "v1"},
				},
			}, nil
		},
	}
	a := &app{client: client}
	meta := &Meta{URL: "https://api.example.com/index", Files: map[string]*File{}}
	err := a.pullIndex(meta)
	if err == nil {
		t.Fatal("expected malformed URL error")
	}
	if !strings.Contains(err.Error(), "invalid bulk resource URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPullIndexKeepsStablePathForSingleEntryCollection(t *testing.T) {
	var out bytes.Buffer
	items := []any{
		map[string]any{"url": "/items/one", "version": "v1"},
		map[string]any{"url": "/items/two", "version": "v2"},
	}
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &out),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			return &httpResponse{Status: 200, Body: items}, nil
		},
	}
	a := &app{client: client}
	meta := &Meta{URL: "https://api.example.com/index", Files: map[string]*File{}}
	if err := a.pullIndex(meta); err != nil {
		t.Fatalf("pullIndex with two entries: %v", err)
	}
	if _, ok := meta.Files["two.json"]; !ok {
		t.Fatalf("two-entry collection did not track two.json: %#v", meta.Files)
	}

	items = []any{map[string]any{"url": "/items/two", "version": "v3"}}
	if err := a.pullIndex(meta); err != nil {
		t.Fatalf("pullIndex with one entry: %v", err)
	}
	if _, ok := meta.Files["two.json"]; !ok {
		t.Fatalf("single-entry collection changed checkout path: %#v", meta.Files)
	}
	if _, ok := meta.Files["items/two.json"]; ok {
		t.Fatalf("single-entry collection created unstable path: %#v", meta.Files)
	}
}

func TestPullIndexResolvesRelativeEntryURLs(t *testing.T) {
	var out bytes.Buffer
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &out),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			return &httpResponse{
				Status: 200,
				Body: []any{
					map[string]any{"url": "/items/one", "version": "v1"},
					map[string]any{"url": "/items/two", "version": "v2"},
				},
			}, nil
		},
	}
	a := &app{client: client}
	meta := &Meta{URL: "https://api.example.com/index", Files: map[string]*File{}}
	if err := a.pullIndex(meta); err != nil {
		t.Fatalf("pullIndex: %v", err)
	}
	if got := meta.Files["one.json"].URL; got != "https://api.example.com/items/one" {
		t.Fatalf("one URL = %q", got)
	}
	if got := meta.Files["two.json"].VersionRemote; got != "v2" {
		t.Fatalf("two version = %q", got)
	}
}

func TestPullIndexResolvesRelativeEntryURLsAgainstResponseURL(t *testing.T) {
	var out bytes.Buffer
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &out),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			if uri != "control/bulk-relative" {
				t.Fatalf("uri = %q", uri)
			}
			return &httpResponse{
				Status: 200,
				URL:    "http://127.0.0.1:8899/bulk/index",
				Body: []any{
					map[string]any{"url": "items/one", "version": "v1"},
				},
			}, nil
		},
	}
	a := &app{client: client}
	meta := &Meta{URL: "control/bulk-relative", Files: map[string]*File{}}
	if err := a.pullIndex(meta); err != nil {
		t.Fatalf("pullIndex: %v", err)
	}
	if got := meta.Files["one.json"].URL; got != "http://127.0.0.1:8899/bulk/items/one" {
		t.Fatalf("one URL = %q", got)
	}
}

func TestSchemaURLUsesDescribedbyLinkAndBodySchema(t *testing.T) {
	if got := schemaURL(&httpResponse{Links: map[string]any{"describedby": "https://api.example.com/schema.json"}}); got != "https://api.example.com/schema.json" {
		t.Fatalf("schema from describedby = %q", got)
	}
	got := schemaURL(&httpResponse{
		URL:  "https://api.example.com/items/one",
		Body: map[string]any{"$schema": "../schemas/item.json"},
	})
	if got != "https://api.example.com/schemas/item.json" {
		t.Fatalf("schema from body = %q", got)
	}
}

func TestCollectFilesIncludesDotPrefixedResources(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".well-known", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".well-known/openid-configuration.json", []byte(`{"issuer":"https://api.example.com"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "ignored.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	meta := &Meta{Files: map[string]*File{}}
	got, err := collectFiles(meta, nil, "", false)
	if err != nil {
		t.Fatalf("collectFiles: %v", err)
	}
	if len(got) != 1 || got[0] != ".well-known/openid-configuration.json" {
		t.Fatalf("collectFiles = %#v, want hidden resource only", got)
	}
}

func TestCollectFilesMatchUsesFileContentTypes(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{
		"high.json":    `{"title":"Restish","rating_average":4.9}`,
		"low.json":     `{"title":"Other","rating_average":4.1}`,
		"missing.json": `{"title":"Missing"}`,
	}
	for name, data := range files {
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	meta := &Meta{Files: map[string]*File{}}

	got, err := collectFiles(meta, nil, "rating_average >= 4.8", false)
	if err != nil {
		t.Fatalf("collectFiles numeric match: %v", err)
	}
	if len(got) != 1 || got[0] != "high.json" {
		t.Fatalf("numeric match = %#v, want [high.json]", got)
	}

	got, err = collectFiles(meta, nil, "title == Restish", false)
	if err != nil {
		t.Fatalf("collectFiles unquoted string match: %v", err)
	}
	if len(got) != 1 || got[0] != "high.json" {
		t.Fatalf("unquoted string match = %#v, want [high.json]", got)
	}
}

func TestCollectFilesMatchWarnsOnSchemaTypeMismatch(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("book.json", []byte(`{"title":"Restish"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	meta := &Meta{Files: map[string]*File{"book.json": {Path: "book.json", Schema: "https://api.example.com/schema.json"}}}
	var warnings []string
	got, err := collectFilesWithOptions(meta, nil, "title > 5", false, func(text string) error {
		warnings = append(warnings, text)
		return nil
	}, func(*File) any {
		return map[string]any{"title": "string"}
	})
	if err != nil {
		t.Fatalf("collectFilesWithOptions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("matched files = %#v, want none", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "cannot compare string with number") {
		t.Fatalf("warnings = %#v, want type mismatch warning", warnings)
	}
}

func TestBulkListFilterReportsInvalidJSONPath(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("broken.json", []byte(`{"title":"Broken"`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	a := &app{client: &pluginClient{CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), out)}}
	meta := &Meta{Files: map[string]*File{}}
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := meta.save(); err != nil {
		t.Fatal(err)
	}

	cmd := a.newListCmd()
	cmd.SetArgs([]string{"--filter", "title"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if !strings.Contains(err.Error(), "broken.json contains invalid JSON") {
		t.Fatalf("error = %v, want filename and invalid JSON context", err)
	}
}

func TestNormalizedBaseURLUsesLocalhostHTTP(t *testing.T) {
	if got, want := normalizedBaseURL("localhost:8080/items"), "http://localhost:8080/items"; got != want {
		t.Fatalf("normalizedBaseURL = %q, want %q", got, want)
	}
}

func TestRenderURLTemplateEscapesPathValues(t *testing.T) {
	item := map[string]any{"id": "a/b?draft=true"}
	got := renderURLTemplate("https://api.example.com/items/{id}", item)
	want := "https://api.example.com/items/a%2Fb%3Fdraft=true"
	if got != want {
		t.Fatalf("renderURLTemplate = %q, want %q", got, want)
	}
}

func TestMetaSaveUsesAtomicRenameAndReplacesExistingFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaFile, []byte(`{"url":"old"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldRename := renameBulkFile
	var sawTemp bool
	renameBulkFile = func(oldpath, newpath string) error {
		if filepath.Dir(oldpath) != metaDir || filepath.Base(newpath) != "meta" {
			t.Fatalf("rename %q -> %q does not use metadata temp path", oldpath, newpath)
		}
		if !strings.Contains(filepath.Base(oldpath), ".meta-") || !strings.HasSuffix(oldpath, ".tmp") {
			t.Fatalf("temp name = %q, want .meta-*.tmp", oldpath)
		}
		if _, err := os.Stat(oldpath); err != nil {
			t.Fatalf("temp file missing before rename: %v", err)
		}
		sawTemp = true
		return oldRename(oldpath, newpath)
	}
	t.Cleanup(func() { renameBulkFile = oldRename })

	meta := &Meta{URL: "https://api.example.com/items", Files: map[string]*File{"one.json": {Path: "one.json", URL: "https://api.example.com/items/one"}}}
	if err := meta.save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !sawTemp {
		t.Fatal("rename hook was not called")
	}
	data, err := os.ReadFile(metaFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "old") || !strings.Contains(string(data), "one.json") {
		t.Fatalf("metadata was not replaced atomically, got %s", data)
	}
	matches, err := filepath.Glob(filepath.Join(metaDir, ".meta-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %#v", matches)
	}
}

func TestPullReturnsMetadataSaveError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	formatted, err := reformat([]byte(`{"id":"one"}`))
	if err != nil {
		t.Fatalf("reformat file: %v", err)
	}
	if err := os.WriteFile("one.json", append(formatted, '\n'), 0o600); err != nil {
		t.Fatalf("write one.json: %v", err)
	}
	oldRename := renameBulkFile
	renameBulkFile = func(oldpath, newpath string) error {
		return fmt.Errorf("rename failed")
	}
	t.Cleanup(func() { renameBulkFile = oldRename })

	var out bytes.Buffer
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &out),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			return &httpResponse{Status: 200, Body: []any{}}, nil
		},
	}
	a := &app{client: client}
	meta := &Meta{
		URL: "https://api.example.com/index",
		Files: map[string]*File{
			"one.json": {Path: "one.json", URL: "https://api.example.com/items/one", VersionLocal: "v1", VersionRemote: "v1", Hash: hashBytes(formatted)},
		},
	}
	err = a.pull(meta, 1)
	if err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("expected metadata save error, got %v", err)
	}
}

func TestFetchFilesHonorsJobsLimit(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	var active atomic.Int32
	var maxActive atomic.Int32
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &out),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			now := active.Add(1)
			for {
				max := maxActive.Load()
				if now <= max || maxActive.CompareAndSwap(max, now) {
					break
				}
			}
			time.Sleep(25 * time.Millisecond)
			active.Add(-1)
			return &httpResponse{
				Status:  200,
				Headers: map[string][]string{"Etag": {uri}},
				Body:    map[string]any{"url": uri},
			}, nil
		},
	}
	a := &app{client: client}
	files := []*File{
		{Path: "one.json", URL: "https://api.example.com/items/one"},
		{Path: "two.json", URL: "https://api.example.com/items/two"},
		{Path: "three.json", URL: "https://api.example.com/items/three"},
		{Path: "four.json", URL: "https://api.example.com/items/four"},
	}

	for result := range a.fetchFiles(files, 2) {
		if result.err != nil {
			t.Fatalf("fetchFiles: %v", result.err)
		}
	}

	if got := maxActive.Load(); got != 2 {
		t.Fatalf("max concurrent requests = %d, want 2", got)
	}
}

func TestPullPersistsMetadataAfterCompletedFileWhenLaterFileFails(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &out),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			switch uri {
			case "https://api.example.com/items":
				return &httpResponse{
					Status: 200,
					Body: []any{
						map[string]any{"url": "https://api.example.com/items/one", "version": "v1"},
						map[string]any{"url": "https://api.example.com/items/two", "version": "v1"},
					},
				}, nil
			case "https://api.example.com/items/one":
				return &httpResponse{Status: 200, Headers: map[string][]string{"Etag": {"one-etag"}}, Body: map[string]any{"id": "one"}}, nil
			case "https://api.example.com/items/two":
				time.Sleep(20 * time.Millisecond)
				return &httpResponse{Status: 500, Body: map[string]any{"error": "boom"}}, nil
			default:
				t.Fatalf("unexpected request %s %s", method, uri)
				return nil, nil
			}
		},
	}
	a := &app{client: client}
	meta := &Meta{URL: "https://api.example.com/items", Files: map[string]*File{}}
	if err := meta.save(); err != nil {
		t.Fatalf("save meta: %v", err)
	}

	if err := a.pull(meta, 2); err == nil {
		t.Fatal("expected pull error")
	}

	data, err := os.ReadFile(metaFile)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var saved Meta
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	one := saved.Files["one.json"]
	if one == nil {
		t.Fatalf("expected one.json metadata in %#v", saved.Files)
	}
	if one.VersionLocal != "v1" || one.VersionRemote != "v1" || one.ETag != "one-etag" {
		t.Fatalf("one.json metadata = %#v, want completed file persisted", one)
	}
	if _, err := os.Stat(filepath.Join(metaDir, "one.json")); err != nil {
		t.Fatalf("expected cached body for one.json: %v", err)
	}
}

func TestPullRemoteDeletedFileWithLocalEditsKeepsMetadataAndPushConflicts(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original, err := reformat([]byte(`{"id":"one","name":"old"}`))
	if err != nil {
		t.Fatalf("reformat original: %v", err)
	}
	if err := os.WriteFile("one.json", []byte(`{"id":"one","name":"local edit"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	meta := &Meta{
		URL: "https://api.example.com/items",
		Files: map[string]*File{
			"one.json": {
				Path:          "one.json",
				URL:           "https://api.example.com/items/one",
				VersionLocal:  "v1",
				VersionRemote: "v1",
				Hash:          hashBytes(original),
			},
		},
	}
	if err := meta.save(); err != nil {
		t.Fatalf("save meta: %v", err)
	}

	var wire bytes.Buffer
	var writes atomic.Int32
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &wire),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			if method != "GET" || uri != "https://api.example.com/items" {
				writes.Add(1)
				return &httpResponse{Status: 200, Body: map[string]any{"id": "one"}}, nil
			}
			return &httpResponse{Status: 200, Body: []any{}}, nil
		},
	}
	a := &app{client: client}
	if err := a.pull(meta, 1); err != nil {
		t.Fatalf("pull remote delete with local edits: %v", err)
	}
	if _, err := os.Stat("one.json"); err != nil {
		t.Fatalf("local edited file should remain: %v", err)
	}
	if _, ok := meta.Files["one.json"]; !ok {
		t.Fatalf("metadata entry was removed: %#v", meta.Files)
	}

	err = a.push(meta, 1, pushOptions{})
	if err == nil {
		t.Fatal("expected push conflict after remote delete")
	}
	if !strings.Contains(err.Error(), "remote was removed") {
		t.Fatalf("expected remote removed conflict, got %v", err)
	}
	if got := writes.Load(); got != 0 {
		t.Fatalf("write requests = %d, want 0", got)
	}
}

func TestPullRemoteDeletedFileRemoveFailurePreservesMetadata(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	formatted, err := reformat([]byte(`{"id":"one"}`))
	if err != nil {
		t.Fatalf("reformat file: %v", err)
	}
	if err := os.WriteFile("one.json", append(formatted, '\n'), 0o600); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	meta := &Meta{
		URL: "https://api.example.com/items",
		Files: map[string]*File{
			"one.json": {
				Path:          "one.json",
				URL:           "https://api.example.com/items/one",
				VersionLocal:  "v1",
				VersionRemote: "v1",
				Hash:          hashBytes(formatted),
			},
		},
	}
	if err := meta.save(); err != nil {
		t.Fatalf("save meta: %v", err)
	}

	oldRemove := removeBulkFile
	removeBulkFile = func(path string) error {
		if path != "one.json" {
			t.Fatalf("remove path = %q, want one.json", path)
		}
		return errors.New("remove denied")
	}
	t.Cleanup(func() { removeBulkFile = oldRemove })

	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			return &httpResponse{Status: 200, Body: []any{}}, nil
		},
	}
	a := &app{client: client}
	err = a.pull(meta, 1)
	if err == nil || !strings.Contains(err.Error(), "remove denied") {
		t.Fatalf("expected remove error, got %v", err)
	}
	if _, err := os.Stat("one.json"); err != nil {
		t.Fatalf("local file should remain after failed remove: %v", err)
	}
	if _, ok := meta.Files["one.json"]; !ok {
		t.Fatalf("metadata entry was removed: %#v", meta.Files)
	}
	data, err := os.ReadFile(metaFile)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	if !strings.Contains(string(data), "one.json") {
		t.Fatalf("saved metadata lost one.json: %s", data)
	}
}

func TestPushFileRejectsVersionConflictWithoutValidator(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("item.json", []byte(`{"id":"item"}`), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	var requests atomic.Int32
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			requests.Add(1)
			return nil, fmt.Errorf("request should not be sent")
		},
	}
	a := &app{client: client}

	_, _, err := a.pushFile(changedFile{
		Status: statusModified,
		File: &File{
			Path:          "item.json",
			URL:           "https://api.example.com/items/item",
			VersionLocal:  "v1",
			VersionRemote: "v2",
		},
	}, pushOptions{})
	if err == nil {
		t.Fatal("expected version conflict")
	}
	if !strings.Contains(err.Error(), "remote version changed") {
		t.Fatalf("expected version conflict error, got %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("request count = %d, want 0", got)
	}
}

func TestDeleteFileRejectsVersionConflictWithoutValidator(t *testing.T) {
	var requests atomic.Int32
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			requests.Add(1)
			return nil, fmt.Errorf("request should not be sent")
		},
	}
	a := &app{client: client}

	_, _, err := a.pushFile(changedFile{
		Status: statusRemoved,
		File: &File{
			Path:          "item.json",
			URL:           "https://api.example.com/items/item",
			VersionLocal:  "v1",
			VersionRemote: "v2",
		},
	}, pushOptions{})
	if err == nil {
		t.Fatal("expected version conflict")
	}
	if !strings.Contains(err.Error(), "remote version changed") {
		t.Fatalf("expected version conflict error, got %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("request count = %d, want 0", got)
	}
}

func TestPushFileRejectsMissingPreconditionWithoutForce(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("item.json", []byte(`{"id":"item"}`), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	var requests atomic.Int32
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			requests.Add(1)
			return nil, fmt.Errorf("request should not be sent")
		},
	}
	a := &app{client: client}

	_, _, err := a.pushFile(changedFile{
		Status: statusModified,
		File: &File{
			Path: "item.json",
			URL:  "https://api.example.com/items/item",
		},
	}, pushOptions{})
	if err == nil {
		t.Fatal("expected missing precondition conflict")
	}
	if !strings.Contains(err.Error(), "no ETag/Last-Modified validator or matching version") {
		t.Fatalf("expected missing precondition error, got %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("request count = %d, want 0", got)
	}
}

func TestPushFileWithETagSendsConditionalHeader(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("item.json", []byte(`{"id":"item"}`), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	var ifMatch string
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			switch method {
			case "PUT":
				ifMatch = headers["If-Match"]
				return &httpResponse{Status: 200, Body: map[string]any{"id": "item"}}, nil
			case "GET":
				return &httpResponse{Status: 200, Body: map[string]any{"id": "item"}}, nil
			default:
				t.Fatalf("unexpected method %s", method)
				return nil, nil
			}
		},
	}
	a := &app{client: client}

	if _, _, err := a.pushFile(changedFile{
		Status: statusModified,
		File: &File{
			Path: "item.json",
			URL:  "https://api.example.com/items/item",
			ETag: `"abc"`,
		},
	}, pushOptions{}); err != nil {
		t.Fatalf("pushFile: %v", err)
	}
	if ifMatch != `"abc"` {
		t.Fatalf("If-Match = %q, want %q", ifMatch, `"abc"`)
	}
}

func TestPushRefusesInvalidTrackedJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("item.json", []byte(`{"id":`), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	formatted, err := reformat([]byte(`{"id":"item"}`))
	if err != nil {
		t.Fatalf("reformat seed: %v", err)
	}
	var writes atomic.Int32
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			if method != "GET" || uri != "https://api.example.com/items" {
				writes.Add(1)
				return nil, fmt.Errorf("unexpected write request %s %s", method, uri)
			}
			return &httpResponse{
				Status: 200,
				Body:   []any{map[string]any{"url": "https://api.example.com/items/item", "version": "v1"}},
			}, nil
		},
	}
	a := &app{client: client}
	meta := &Meta{
		URL: "https://api.example.com/items",
		Files: map[string]*File{
			"item.json": {
				Path:          "item.json",
				URL:           "https://api.example.com/items/item",
				VersionLocal:  "v1",
				VersionRemote: "v1",
				Hash:          hashBytes(formatted),
			},
		},
	}

	err = a.push(meta, 1, pushOptions{})
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
	if got := writes.Load(); got != 0 {
		t.Fatalf("write requests = %d, want 0", got)
	}
}

func TestApplyFetchedFileClearsStaleValidators(t *testing.T) {
	f := &File{ETag: `"old"`, LastModified: "yesterday"}
	applyFetchedFile(f, &fetchedFile{})
	if f.ETag != "" || f.LastModified != "" {
		t.Fatalf("validators = etag:%q last-modified:%q, want cleared", f.ETag, f.LastModified)
	}
}

func TestPushFileMatchingVersionWithoutValidatorSucceeds(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("item.json", []byte(`{"id":"item"}`), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	var putSeen bool
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			switch method {
			case "PUT":
				putSeen = true
				if len(headers) != 0 {
					t.Fatalf("unexpected conditional headers for version-only push: %#v", headers)
				}
				return &httpResponse{Status: 200, Body: map[string]any{"id": "item"}}, nil
			case "GET":
				return &httpResponse{Status: 200, Body: map[string]any{"id": "item"}}, nil
			default:
				t.Fatalf("unexpected method %s", method)
				return nil, nil
			}
		},
	}
	a := &app{client: client}

	if _, _, err := a.pushFile(changedFile{
		Status: statusModified,
		File: &File{
			Path:          "item.json",
			URL:           "https://api.example.com/items/item",
			VersionLocal:  "v1",
			VersionRemote: "v1",
		},
	}, pushOptions{}); err != nil {
		t.Fatalf("pushFile: %v", err)
	}
	if !putSeen {
		t.Fatal("expected PUT request")
	}
}

func TestPushFileForcePermitsMissingPrecondition(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("item.json", []byte(`{"id":"item"}`), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	var gotHeaders map[string]string
	client := &pluginClient{
		CommandClient: pluginwire.NewCommandClient(bytes.NewReader(nil), &bytes.Buffer{}),
		requestFunc: func(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
			gotHeaders = headers
			switch method {
			case "PUT":
				return &httpResponse{Status: 200, Body: map[string]any{"id": "item"}}, nil
			case "GET":
				return &httpResponse{Status: 200, Body: map[string]any{"id": "item"}}, nil
			default:
				t.Fatalf("unexpected method %s", method)
				return nil, nil
			}
		},
	}
	a := &app{client: client}

	if _, _, err := a.pushFile(changedFile{
		Status: statusModified,
		File: &File{
			Path: "item.json",
			URL:  "https://api.example.com/items/item",
		},
	}, pushOptions{Force: true}); err != nil {
		t.Fatalf("pushFile force: %v", err)
	}
	if len(gotHeaders) != 0 {
		t.Fatalf("force should not invent precondition headers, got %#v", gotHeaders)
	}
}
