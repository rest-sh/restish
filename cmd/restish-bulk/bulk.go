package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/danielgtaylor/shorthand/v2"
)

type fileStatus uint8

const (
	statusAdded fileStatus = iota + 1
	statusModified
	statusRemoved
)

type changedFile struct {
	Status fileStatus
	File   *File
}

type pushOptions struct {
	Force bool
}

func (a *app) pullIndex(m *Meta) error {
	resp, err := a.client.request("GET", m.URL, nil, nil)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s", resp.Error)
	}
	if resp.Status >= 400 {
		_ = a.client.response(resp)
		return fmt.Errorf("error fetching %s", m.URL)
	}

	data := resp.Body
	if m.Filter != "" {
		result, _, err := shorthand.GetPath(m.Filter, map[string]any{
			"status":  resp.Status,
			"headers": resp.Headers,
			"body":    resp.Body,
		}, shorthand.GetOptions{})
		if err != nil {
			return err
		}
		data = result
	}

	items, ok := data.([]any)
	if !ok {
		return fmt.Errorf("resource list response is not a list")
	}

	entries := make([]listEntry, 0, len(items))
	for _, item := range items {
		rawURL := getFirstKey(item, "url", "uri", "self", "link")
		if rawURL == "" && m.URLTemplate != "" {
			rawURL = renderURLTemplate(m.URLTemplate, item)
		}
		version := getFirstKey(item, "version", "etag", "last_modified", "lastModified", "modified")
		if rawURL == "" || version == "" {
			return fmt.Errorf("list response must contain a URL and version for each resource")
		}
		entries = append(entries, listEntry{URL: rawURL, Version: version})
	}

	baseURL, err := url.Parse(normalizedBaseURL(m.URL))
	if err != nil {
		return fmt.Errorf("invalid bulk index URL %q: %w", m.URL, err)
	}
	m.Base = commonPrefix(baseURL, entries)

	for _, f := range m.Files {
		f.VersionRemote = ""
	}
	for _, entry := range entries {
		resolved, err := resolveBulkEntryURL(baseURL, entry.URL)
		if err != nil {
			return err
		}
		relPath, err := bulkRelativePath(m.Base, resolved)
		if err != nil {
			return err
		}
		f := m.Files[relPath]
		if f == nil {
			f = &File{Path: relPath, URL: resolved}
			m.Files[relPath] = f
		}
		f.VersionRemote = entry.Version
	}
	return nil
}

func resolveBulkEntryURL(baseURL *url.URL, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid bulk resource URL %q: %w", raw, err)
	}
	if u == nil {
		return "", fmt.Errorf("invalid bulk resource URL %q", raw)
	}
	return baseURL.ResolveReference(u).String(), nil
}

func (a *app) pull(m *Meta, jobs int) error {
	if err := a.pullIndex(m); err != nil {
		return err
	}
	jobs = normalizeJobs(jobs)

	updates := []*File{}
	for _, f := range m.Files {
		if f.VersionLocal != "" && f.VersionLocal == f.VersionRemote {
			continue
		}
		updates = append(updates, f)
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].Path < updates[j].Path })

	if len(updates) == 0 {
		return a.client.WriteStdout([]byte("Already up to date.\n"))
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Pulling %d resource(s)...\n", len(updates))
	if err := a.client.WriteStderr(buf.Bytes()); err != nil {
		return err
	}

	var firstErr error
	fetches := make([]*File, 0, len(updates))
	for _, f := range updates {
		if f.VersionRemote == "" {
			delete(m.Files, f.Path)
			if err := m.save(); err != nil {
				return err
			}
			changed, err := f.isChangedLocal(true)
			if err != nil {
				if warnErr := a.client.Warn("skipping delete due to invalid local JSON: " + f.Path); warnErr != nil && firstErr == nil {
					firstErr = warnErr
				}
				if firstErr == nil {
					firstErr = err
				}
			} else if !changed {
				_ = os.Remove(f.Path)
			}
			continue
		}
		fetches = append(fetches, f)
	}

	results := a.fetchFiles(fetches, jobs)
	for result := range results {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		f := result.file
		applyFetchedFile(f, result.fetched)
		if err := m.save(); err != nil {
			return err
		}
		changed, err := f.isChangedLocal(true)
		if err != nil {
			if warnErr := a.client.Warn("skipping due to invalid local JSON: " + f.Path); warnErr != nil && firstErr == nil {
				firstErr = warnErr
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if changed {
			if err := a.client.Warn("skipping due to local edits: " + f.Path); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := f.write(result.fetched.body); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		f.VersionLocal = f.VersionRemote
		if err := m.save(); err != nil {
			return err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return m.save()
}

type pullFetchResult struct {
	file    *File
	fetched *fetchedFile
	err     error
}

func (a *app) fetchFiles(files []*File, jobs int) <-chan pullFetchResult {
	results := make(chan pullFetchResult)
	if len(files) == 0 {
		close(results)
		return results
	}
	go func() {
		defer close(results)
		jobs = min(jobs, len(files))
		work := make(chan *File)
		var wg sync.WaitGroup
		for range jobs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for f := range work {
					body, err := a.fetchFileData(f)
					results <- pullFetchResult{file: f, fetched: body, err: err}
				}
			}()
		}
		for _, f := range files {
			work <- f
		}
		close(work)
		wg.Wait()
	}()
	return results
}

func (a *app) push(m *Meta, jobs int, opts pushOptions) error {
	jobs = normalizeJobs(jobs)
	paths, err := collectFiles(m, nil, "", false)
	if err != nil {
		return err
	}
	local, remote, err := a.getChanged(m, paths)
	if err != nil {
		return err
	}
	sort.Slice(local, func(i, j int) bool { return local[i].File.Path < local[j].File.Path })

	var firstErr error
	var summary pushSummary
	remoteByPath := make(map[string]changedFile, len(remote))
	for _, changed := range remote {
		if changed.File != nil {
			remoteByPath[changed.File.Path] = changed
		}
	}
	pushable := local[:0]
	for _, changed := range local {
		if changed.File != nil {
			if remoteChanged, ok := remoteByPath[changed.File.Path]; ok {
				summary.Refused++
				err := remoteLocalConflictError(changed, remoteChanged)
				if warnErr := a.client.Warn(err.Error()); warnErr != nil && firstErr == nil {
					firstErr = warnErr
				}
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
		}
		pushable = append(pushable, changed)
	}

	results := a.pushFiles(pushable, jobs, opts)
	for result := range results {
		if result.err != nil {
			summary.Refused++
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		changed := result.changed
		f := changed.File
		switch changed.Status {
		case statusAdded, statusModified:
			if changed.Status == statusAdded {
				summary.Created++
			} else {
				summary.Updated++
			}
			if changed.Status == statusAdded {
				m.Files[f.Path] = f
			}
			if len(result.hash) > 0 {
				f.Hash = result.hash
				if err := m.save(); err != nil {
					return err
				}
			}
			applyFetchedFile(f, result.fetched)
			if err := f.write(result.fetched.body); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if err := m.save(); err != nil {
				return err
			}
		case statusRemoved:
			summary.Deleted++
			delete(m.Files, f.Path)
			if err := m.save(); err != nil {
				return err
			}
		}
	}
	_ = a.writePushSummary(summary)
	if firstErr != nil {
		return firstErr
	}

	if err := a.pullIndex(m); err != nil {
		return err
	}
	for _, changed := range local {
		if changed.File != nil {
			changed.File.VersionLocal = changed.File.VersionRemote
		}
	}
	return m.save()
}

func remoteLocalConflictError(local, remote changedFile) error {
	action := "changed"
	if remote.Status == statusRemoved {
		action = "removed"
	}
	path := ""
	if local.File != nil {
		path = local.File.Path
	}
	return fmt.Errorf("conflict pushing %s: remote was %s while local file has edits; pull and review before pushing", path, action)
}

type pushSummary struct {
	Created int
	Updated int
	Deleted int
	Skipped int
	Refused int
}

func (a *app) writePushSummary(s pushSummary) error {
	return a.client.WriteStdout([]byte(fmt.Sprintf(
		"Push summary: created=%d updated=%d deleted=%d skipped=%d refused=%d\n",
		s.Created, s.Updated, s.Deleted, s.Skipped, s.Refused,
	)))
}

type pushResult struct {
	changed changedFile
	fetched *fetchedFile
	hash    []byte
	err     error
}

func (a *app) pushFiles(changes []changedFile, jobs int, opts pushOptions) <-chan pushResult {
	results := make(chan pushResult)
	if len(changes) == 0 {
		close(results)
		return results
	}
	go func() {
		defer close(results)
		jobs = min(jobs, len(changes))
		work := make(chan changedFile)
		var wg sync.WaitGroup
		for range jobs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for changed := range work {
					fetched, hash, err := a.pushFile(changed, opts)
					results <- pushResult{changed: changed, fetched: fetched, hash: hash, err: err}
				}
			}()
		}
		for _, changed := range changes {
			work <- changed
		}
		close(work)
		wg.Wait()
	}()
	return results
}

func (a *app) pushFile(changed changedFile, opts pushOptions) (*fetchedFile, []byte, error) {
	f := changed.File
	switch changed.Status {
	case statusAdded, statusModified:
		body, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, nil, err
		}
		payload, err := decodeJSON(body)
		if err != nil {
			return nil, nil, err
		}
		headers := preconditionHeaders(f)
		if changed.Status == statusModified && len(headers) == 0 && !opts.Force {
			if !versionPreconditionSatisfied(f) {
				return nil, nil, missingPreconditionError("uploading", f)
			}
		}
		resp, err := a.client.request("PUT", f.URL, headers, payload)
		if err != nil {
			return nil, nil, err
		}
		if resp.Error != "" {
			return nil, nil, fmt.Errorf("%s", resp.Error)
		}
		if resp.Status >= 400 {
			_ = a.client.response(resp)
			return nil, nil, fmt.Errorf("error uploading %s", f.Path)
		}
		var localHash []byte
		if formatted, err := reformat(body); err == nil {
			localHash = hashBytes(formatted)
		}
		remoteBody, err := a.fetchFileData(f)
		if err != nil {
			return nil, nil, err
		}
		return remoteBody, localHash, nil
	case statusRemoved:
		headers := preconditionHeaders(f)
		if len(headers) == 0 && !opts.Force {
			if !versionPreconditionSatisfied(f) {
				return nil, nil, missingPreconditionError("deleting", f)
			}
		}
		resp, err := a.client.request("DELETE", f.URL, headers, nil)
		if err != nil {
			return nil, nil, err
		}
		if resp.Error != "" {
			return nil, nil, fmt.Errorf("%s", resp.Error)
		}
		if resp.Status >= 400 {
			_ = a.client.response(resp)
			return nil, nil, fmt.Errorf("error deleting %s", f.Path)
		}
		return nil, nil, nil
	}
	return nil, nil, nil
}

func preconditionHeaders(f *File) map[string]string {
	headers := map[string]string{}
	if f == nil {
		return headers
	}
	if f.ETag != "" {
		headers["If-Match"] = f.ETag
	} else if f.LastModified != "" {
		headers["If-Unmodified-Since"] = f.LastModified
	}
	return headers
}

func versionConflictWithoutValidator(f *File) bool {
	if f == nil {
		return false
	}
	if f.VersionLocal == "" && f.VersionRemote == "" {
		return false
	}
	return f.VersionLocal != f.VersionRemote
}

func versionConflictError(action string, f *File) error {
	return fmt.Errorf("conflict %s %s: remote version changed from %q to %q and no ETag/Last-Modified validator is available; pull and review before pushing", action, f.Path, f.VersionLocal, f.VersionRemote)
}

func versionPreconditionSatisfied(f *File) bool {
	if f == nil {
		return false
	}
	if f.VersionLocal != "" || f.VersionRemote != "" {
		return f.VersionLocal == f.VersionRemote
	}
	return false
}

func missingPreconditionError(action string, f *File) error {
	if versionConflictWithoutValidator(f) {
		return versionConflictError(action, f)
	}
	return fmt.Errorf("conflict %s %s: no ETag/Last-Modified validator or matching version is available; pull and review before pushing, or pass --force", action, f.Path)
}

func (a *app) getChanged(m *Meta, files []string) ([]changedFile, []changedFile, error) {
	if err := a.pullIndex(m); err != nil {
		return nil, nil, err
	}

	filesMap := map[string]bool{}
	for _, path := range files {
		filesMap[path] = true
	}

	local := []changedFile{}
	remote := []changedFile{}
	for _, path := range files {
		if f, ok := m.Files[path]; ok {
			changed, err := f.isChangedLocal(true)
			if err != nil {
				return nil, nil, err
			}
			if changed {
				local = append(local, changedFile{Status: statusModified, File: f})
			}
			if f.VersionRemote == "" {
				remote = append(remote, changedFile{Status: statusRemoved, File: f})
			} else if f.VersionLocal != f.VersionRemote {
				remote = append(remote, changedFile{Status: statusModified, File: f})
			}
		} else {
			local = append(local, changedFile{
				Status: statusAdded,
				File: &File{
					Path: path,
					URL:  m.Base + strings.TrimSuffix(path, filepath.Ext(path)),
				},
			})
		}
	}

	for _, f := range m.Files {
		if f.VersionLocal == "" {
			remote = append(remote, changedFile{Status: statusAdded, File: f})
			continue
		}
		if !filesMap[f.Path] {
			local = append(local, changedFile{Status: statusRemoved, File: f})
		}
	}

	sort.Slice(local, func(i, j int) bool { return local[i].File.Path < local[j].File.Path })
	sort.Slice(remote, func(i, j int) bool { return remote[i].File.Path < remote[j].File.Path })
	return local, remote, nil
}

func (a *app) localDiff(meta *Meta, files []string) error {
	changed := false
	for _, path := range files {
		var original []byte
		if f, ok := meta.Files[path]; ok {
			changed, err := f.isChangedLocal(false)
			if err != nil {
				return err
			}
			if !changed {
				continue
			}
			original, err = os.ReadFile(filepath.Join(metaDir, filepath.FromSlash(path)))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		modified, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		changed = true
		diff := unifiedDiff("remote "+meta.Base+strings.TrimSuffix(path, ".json"), "local "+path, original, modified)
		if err := a.client.WriteStdout([]byte(diff)); err != nil {
			return err
		}
	}
	if !changed {
		return a.client.WriteStdout([]byte("No local changes\n"))
	}
	return nil
}

func (a *app) remoteDiff(meta *Meta) error {
	paths, err := collectFiles(meta, nil, "", true)
	if err != nil {
		return err
	}
	_, remote, err := a.getChanged(meta, paths)
	if err != nil {
		return err
	}
	if len(remote) == 0 {
		return a.client.WriteStdout([]byte("No remote changes\n"))
	}
	for _, changed := range remote {
		original, err := os.ReadFile(changed.File.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		var modified []byte
		switch changed.Status {
		case statusAdded, statusModified:
			modified, err = a.fetchFile(changed.File)
			if err != nil {
				return err
			}
		case statusRemoved:
			modified = nil
		}
		diff := unifiedDiff("local "+changed.File.Path, "remote "+meta.Base+strings.TrimSuffix(changed.File.Path, ".json"), original, modified)
		if err := a.client.WriteStdout([]byte(diff)); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) fetchFile(f *File) ([]byte, error) {
	fetched, err := a.fetchFileData(f)
	if err != nil {
		return nil, err
	}
	applyFetchedFile(f, fetched)
	return fetched.body, nil
}

type fetchedFile struct {
	body         []byte
	etag         string
	lastModified string
}

func (a *app) fetchFileData(f *File) (*fetchedFile, error) {
	resp, err := a.client.request("GET", f.URL, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	if resp.Status >= 400 {
		_ = a.client.response(resp)
		return nil, fmt.Errorf("error fetching %s", f.URL)
	}
	body, err := prettyJSON(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := f.writeCached(body); err != nil {
		return nil, err
	}
	return &fetchedFile{
		body:         append(body, '\n'),
		etag:         firstHeader(resp.Headers, "Etag"),
		lastModified: firstHeader(resp.Headers, "Last-Modified"),
	}, nil
}

func firstHeader(headers map[string][]string, name string) string {
	for key, values := range headers {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func applyFetchedFile(f *File, fetched *fetchedFile) {
	if fetched == nil {
		return
	}
	f.ETag = fetched.etag
	f.LastModified = fetched.lastModified
}

func normalizeJobs(jobs int) int {
	if jobs < 1 {
		return defaultJobs
	}
	return jobs
}
