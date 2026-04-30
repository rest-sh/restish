package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pluginwire "github.com/rest-sh/restish/v2/plugin"
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
				Headers: map[string]string{"Etag": uri},
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
				return &httpResponse{Status: 200, Headers: map[string]string{"Etag": "one-etag"}, Body: map[string]any{"id": "one"}}, nil
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
