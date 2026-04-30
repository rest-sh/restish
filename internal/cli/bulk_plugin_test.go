package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type bulkItem struct {
	User    string         `json:"user"`
	ID      string         `json:"id"`
	Version string         `json:"version"`
	Body    map[string]any `json:"-"`
	ETag    string         `json:"-"`
}

type bulkServer struct {
	server *httptest.Server
	mu     sync.Mutex
	items  map[string]*bulkItem
}

type bulkListEntry struct {
	User    string `json:"user"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

func newBulkServer(t *testing.T, items []*bulkItem) *bulkServer {
	t.Helper()
	s := &bulkServer{items: map[string]*bulkItem{}}
	for _, item := range items {
		s.items[itemPath(item.User, item.ID)] = item
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *bulkServer) listURL() string {
	return s.server.URL + "/all-items"
}

func (s *bulkServer) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/all-items":
		entries := make([]bulkListEntry, 0, len(s.items))
		for _, item := range s.items {
			entries = append(entries, bulkListEntry{User: item.User, ID: item.ID, Version: item.Version})
		}
		sortEntries(entries)
		writeJSON(w, entries)
	case strings.HasPrefix(r.URL.Path, "/users/"):
		key := strings.TrimPrefix(r.URL.Path, "/users/")
		parts := strings.Split(key, "/items/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		user, id := parts[0], parts[1]
		key = itemPath(user, id)
		item := s.items[key]
		switch r.Method {
		case http.MethodGet:
			if item == nil {
				http.NotFound(w, r)
				return
			}
			if item.ETag != "" {
				w.Header().Set("Etag", item.ETag)
			}
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			writeJSON(w, item.Body)
		case http.MethodPut:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			version := fmt.Sprintf("%s-v%d", id, len(body))
			s.items[key] = &bulkItem{
				User:    user,
				ID:      id,
				Version: version,
				Body:    body,
				ETag:    version,
			}
			writeJSON(w, map[string]any{"ok": true})
		case http.MethodDelete:
			if item == nil {
				http.NotFound(w, r)
				return
			}
			delete(s.items, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func sortEntries(entries []bulkListEntry) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].User < entries[i].User || (entries[j].User == entries[i].User && entries[j].ID < entries[i].ID) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

func itemPath(user, id string) string {
	return user + "/items/" + id
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func installBulkPlugin(t *testing.T) {
	t.Helper()
	skipNoBulkPlugin(t)

	pluginsParent, _ := installSharedPlugin(t, "bulk", testBulkPluginBin, "restish-bulk")
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	t.Setenv("PATH", "")
}

func withWorkingDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	return dir
}

func TestBulkPluginHelpAndDiscovery(t *testing.T) {
	installBulkPlugin(t)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out.String(), "bulk") {
		t.Fatalf("expected bulk command in help, got:\n%s", out.String())
	}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "--help"}); err != nil {
		t.Fatalf("bulk help: %v", err)
	}
	if !strings.Contains(out.String(), "init") || !strings.Contains(out.String(), "push") {
		t.Fatalf("expected plugin-managed subcommands in help, got:\n%s", out.String())
	}
}

func TestBulkPluginWorkflow(t *testing.T) {
	installBulkPlugin(t)
	withWorkingDir(t)

	srv := newBulkServer(t, []*bulkItem{
		{User: "a", ID: "a1", Version: "a11", Body: map[string]any{"id": "a1"}, ETag: "a11"},
		{User: "a", ID: "a2", Version: "a21", Body: map[string]any{"id": "a2"}, ETag: "a21"},
		{User: "b", ID: "b1", Version: "b11", Body: map[string]any{"id": "b1"}, ETag: "b11"},
		{User: "c", ID: "c1", Version: "c11", Body: map[string]any{"id": "c1"}, ETag: "c11"},
	})

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")

	if err := c.Run([]string{"restish", "bulk", "init", srv.listURL(), "--url-template=/users/{user}/items/{id}"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, path := range []string{
		"a/items/a1.json",
		"a/items/a2.json",
		"b/items/b1.json",
		"c/items/c1.json",
		".rshbulk/meta",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "list", "-m", "id contains 1"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "a/items/a1.json") || strings.Contains(got, "a/items/a2.json") {
		t.Fatalf("unexpected list output:\n%s", got)
	}

	srv.items[itemPath("b", "b1")].Version = "b12"
	srv.items[itemPath("b", "b1")].Body = map[string]any{"id": "b1", "foo": 1}
	delete(srv.items, itemPath("c", "c1"))
	srv.items[itemPath("d", "d1")] = &bulkItem{User: "d", ID: "d1", Version: "d11", Body: map[string]any{"id": "d1"}, ETag: "d11"}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "status"}); err != nil {
		t.Fatalf("status: %v", err)
	}
	got = out.String()
	for _, want := range []string{"Remote changes", "modified:  b/items/b1.json", "removed:  c/items/c1.json", "added:  d/items/d1.json"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in status output:\n%s", want, got)
		}
	}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "diff", "--remote"}); err != nil {
		t.Fatalf("remote diff: %v", err)
	}
	got = out.String()
	if !strings.Contains(got, "+  \"foo\": 1") || !strings.Contains(got, "+  \"id\": \"d1\"") {
		t.Fatalf("unexpected remote diff output:\n%s", got)
	}

	if err := c.Run([]string{"restish", "bulk", "pull"}); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if _, err := os.Stat("d/items/d1.json"); err != nil {
		t.Fatalf("expected pulled file: %v", err)
	}
	if _, err := os.Stat("c/items/c1.json"); !os.IsNotExist(err) {
		t.Fatalf("expected removed file to be deleted, got: %v", err)
	}

	if err := os.WriteFile("a/items/a1.json", []byte("{\n  \"id\": \"a1\",\n  \"name\": \"alice\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("d/items/d1.json", []byte("{\n  \"id\": \"d1\",\n  \"extra\": true\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove("b/items/b1.json"); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "status"}); err != nil {
		t.Fatalf("status after local edits: %v", err)
	}
	got = out.String()
	for _, want := range []string{"modified:  a/items/a1.json", "removed:  b/items/b1.json"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in local status output:\n%s", want, got)
		}
	}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "diff"}); err != nil {
		t.Fatalf("local diff: %v", err)
	}
	got = out.String()
	if !strings.Contains(got, "+  \"name\": \"alice\"") || !strings.Contains(got, "-  \"id\": \"b1\"") {
		t.Fatalf("unexpected local diff output:\n%s", got)
	}

	if err := c.Run([]string{"restish", "bulk", "push"}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if _, ok := srv.items[itemPath("a", "a1")].Body["name"]; !ok {
		t.Fatalf("expected remote modification to be pushed")
	}
	if _, ok := srv.items[itemPath("b", "b1")]; ok {
		t.Fatalf("expected remote delete to be pushed")
	}
	if value := srv.items[itemPath("d", "d1")].Body["extra"]; value != true {
		t.Fatalf("expected added file to be pushed, got %#v", srv.items[itemPath("d", "d1")].Body)
	}
}

func TestBulkPluginRemoteDeleteLocalEditConflictIsSafe(t *testing.T) {
	installBulkPlugin(t)
	withWorkingDir(t)

	srv := newBulkServer(t, []*bulkItem{
		{User: "a", ID: "a1", Version: "a11", Body: map[string]any{"id": "a1"}},
		{User: "b", ID: "b1", Version: "b11", Body: map[string]any{"id": "b1"}},
	})

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	if err := c.Run([]string{"restish", "bulk", "init", srv.listURL(), "--url-template=/users/{user}/items/{id}"}); err != nil {
		t.Fatalf("init: %v", err)
	}

	delete(srv.items, itemPath("a", "a1"))
	if err := os.WriteFile("a/items/a1.json", []byte("{\n  \"id\": \"a1\",\n  \"name\": \"local edit\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "status"}); err != nil {
		t.Fatalf("status: %v", err)
	}
	status := out.String()
	for _, want := range []string{"Local changes", "modified:  a/items/a1.json", "Remote changes", "removed:  a/items/a1.json"} {
		if !strings.Contains(status, want) {
			t.Fatalf("status missing %q:\n%s", want, status)
		}
	}

	out.Reset()
	errOut.Reset()
	err := c.Run([]string{"restish", "bulk", "push"})
	if err == nil {
		t.Fatal("expected push conflict")
	}
	if _, ok := srv.items[itemPath("a", "a1")]; ok {
		t.Fatal("remote item was recreated despite conflict")
	}
	if !strings.Contains(out.String(), "refused=1") {
		t.Fatalf("expected refused summary, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "remote was removed") {
		t.Fatalf("expected conflict warning, got:\n%s", errOut.String())
	}
}

func TestBulkPullKeepsRemoteChangeWhenLocalEditsBlockWrite(t *testing.T) {
	installBulkPlugin(t)
	withWorkingDir(t)

	srv := newBulkServer(t, []*bulkItem{
		{User: "a", ID: "a1", Version: "a11", Body: map[string]any{"id": "a1"}, ETag: "a11"},
		{User: "b", ID: "b1", Version: "b11", Body: map[string]any{"id": "b1"}, ETag: "b11"},
	})

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")

	if err := c.Run([]string{"restish", "bulk", "init", srv.listURL(), "--url-template=/users/{user}/items/{id}"}); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile("a/items/a1.json", []byte("{\n  \"id\": \"a1\",\n  \"name\": \"local\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv.items[itemPath("a", "a1")].Version = "a12"
	srv.items[itemPath("a", "a1")].Body = map[string]any{"id": "a1", "name": "remote"}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "pull"}); err != nil {
		t.Fatalf("pull: %v", err)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "bulk", "status"}); err != nil {
		t.Fatalf("status: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Remote changes") || !strings.Contains(got, "modified:  a/items/a1.json") {
		t.Fatalf("expected remote modification to remain pending, got:\n%s", got)
	}
}
