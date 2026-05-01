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
	"sync"

	"github.com/danielgtaylor/mexpr"
	"github.com/danielgtaylor/shorthand/v2"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/spf13/cobra"
)

const (
	metaDir     = ".rshbulk"
	metaFile    = ".rshbulk/meta"
	defaultJobs = 4
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

type app struct {
	client *pluginClient
}

func run(client *pluginClient, args []string) error {
	a := &app{client: client}
	root := a.newRootCmd()
	root.SetArgs(args)
	root.SetOut(client.StdoutWriter())
	root.SetErr(client.StderrWriter())
	return root.Execute()
}

func (a *app) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "bulk",
		Short:         "Client-side bulk resource management",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(a.newInitCmd())
	root.AddCommand(a.newListCmd())
	root.AddCommand(a.newStatusCmd())
	root.AddCommand(a.newDiffCmd())
	root.AddCommand(a.newResetCmd())
	root.AddCommand(a.newPullCmd())
	root.AddCommand(a.newPushCmd())
	return root
}

func (a *app) newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init URL",
		Aliases: []string{"i"},
		Short:   "Initialize a new bulk checkout",
		Long:    "Initialize a bulk checkout from a list endpoint that returns each resource URL and version, optionally transforming the list with -f and --url-template first.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter, _ := cmd.Flags().GetString("filter")
			template, _ := cmd.Flags().GetString("url-template")
			jobs, _ := cmd.Flags().GetInt("jobs")
			meta := &Meta{
				URL:         args[0],
				Filter:      filter,
				URLTemplate: template,
				Files:       map[string]*File{},
			}
			if err := meta.save(); err != nil {
				return err
			}
			return a.pull(meta, jobs)
		},
	}
	cmd.Flags().StringP("filter", "f", "", "Filter/project the list response before extracting url/version")
	cmd.Flags().String("url-template", "", "URL template to build resource links, e.g. /users/{id}")
	cmd.Flags().IntP("jobs", "j", defaultJobs, "Maximum concurrent resource requests")
	return cmd
}

func (a *app) newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List checked out files",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			filterExpr, _ := cmd.Flags().GetString("filter")
			files, err := collectFiles(meta, nil, match, false)
			if err != nil {
				return err
			}
			for _, path := range files {
				if err := a.client.WriteStdout([]byte(path + "\n")); err != nil {
					return err
				}
				if filterExpr == "" {
					continue
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				var content any
				if err := json.Unmarshal(data, &content); err != nil {
					return err
				}
				res, _, err := shorthand.GetPath(filterExpr, content, shorthand.GetOptions{})
				if err != nil || isFalsey(res) {
					continue
				}
				formatted, err := prettyJSON(res)
				if err != nil {
					return err
				}
				if err := a.client.WriteStdout(append(formatted, '\n')); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringP("match", "m", "", "Expression to match")
	cmd.Flags().StringP("filter", "f", "", "Show projected content for each matched file")
	return cmd
}

func (a *app) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "Show local and remote added/changed/removed files",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			files, err := collectFiles(meta, nil, "", false)
			if err != nil {
				return err
			}
			local, remote, err := a.getChanged(meta, files)
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			if len(remote) > 0 {
				fmt.Fprintf(&buf, "Remote changes on %s\n  (use \"restish bulk pull\" to update)\n", normalizedBaseURL(meta.URL))
				for _, changed := range remote {
					fmt.Fprintln(&buf, changed.String())
				}
			} else {
				fmt.Fprintf(&buf, "You are up to date with %s\n", normalizedBaseURL(meta.URL))
			}
			if len(local) == 0 {
				fmt.Fprintln(&buf, "No local changes")
			} else {
				fmt.Fprintln(&buf, "Local changes:")
				fmt.Fprintln(&buf, "  (use \"restish bulk reset [file]...\" to undo)")
				fmt.Fprintln(&buf, "  (use \"restish bulk diff [file]...\" to view changes)")
				for _, changed := range local {
					fmt.Fprintln(&buf, changed.String())
				}
			}
			return a.client.WriteStdout(buf.Bytes())
		},
	}
}

func (a *app) newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diff [file...]",
		Aliases: []string{"di"},
		Short:   "Show local or remote diffs",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			remote, _ := cmd.Flags().GetBool("remote")
			if remote {
				return a.remoteDiff(meta)
			}
			files, err := collectFiles(meta, args, match, true)
			if err != nil {
				return err
			}
			return a.localDiff(meta, files)
		},
	}
	cmd.Flags().StringP("match", "m", "", "Expression to match")
	cmd.Flags().Bool("remote", false, "Show remote diffs instead of local")
	return cmd
}

func (a *app) newResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reset [file...]",
		Aliases: []string{"re"},
		Short:   "Undo local changes to files",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			files, err := collectFiles(meta, args, match, true)
			if err != nil {
				return err
			}
			for _, name := range files {
				f := meta.Files[name]
				if f == nil || f.VersionLocal == "" {
					continue
				}
				if err := f.reset(); err != nil {
					return err
				}
			}
			return meta.save()
		},
	}
	cmd.Flags().StringP("match", "m", "", "Expression to match")
	return cmd
}

func (a *app) newPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pull",
		Aliases: []string{"pl"},
		Short:   "Pull remote updates without overwriting local changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			jobs, _ := cmd.Flags().GetInt("jobs")
			return a.pull(meta, jobs)
		},
	}
	cmd.Flags().IntP("jobs", "j", defaultJobs, "Maximum concurrent resource requests")
	return cmd
}

func (a *app) newPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "push",
		Aliases: []string{"ps"},
		Short:   "Upload local changes to the remote server",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			jobs, _ := cmd.Flags().GetInt("jobs")
			force, _ := cmd.Flags().GetBool("force")
			return a.push(meta, jobs, pushOptions{Force: force})
		},
	}
	cmd.Flags().IntP("jobs", "j", defaultJobs, "Maximum concurrent resource requests")
	cmd.Flags().Bool("force", false, "Push without ETag/Last-Modified or matching version preconditions")
	return cmd
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
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(metaFile, append(data, '\n'), 0o600)
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

	baseURL, _ := url.Parse(normalizedBaseURL(m.URL))
	m.Base = commonPrefix(baseURL, entries)

	for _, f := range m.Files {
		f.VersionRemote = ""
	}
	for _, entry := range entries {
		u, _ := url.Parse(entry.URL)
		resolved := baseURL.ResolveReference(u).String()
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
			_ = m.save()
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
		_ = m.save()
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
		_ = m.save()
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
				_ = m.save()
			}
			applyFetchedFile(f, result.fetched)
			if err := f.write(result.fetched.body); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			_ = m.save()
		case statusRemoved:
			summary.Deleted++
			delete(m.Files, f.Path)
			_ = m.save()
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
		if strings.HasPrefix(path, ".") {
			continue
		}
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
				if strings.HasPrefix(filepath.Base(path), ".") && path != "." {
					return filepath.SkipDir
				}
				return nil
			}
			rel := filepath.ToSlash(path)
			if strings.HasPrefix(rel, ".") {
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
		ast, err := mexpr.Parse(match, map[string]any{}, mexpr.UnquotedStrings)
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
		return fmt.Sprintf("%v", m[key])
	})
}

func normalizedBaseURL(raw string) string {
	if strings.Contains(raw, "://") {
		return raw
	}
	if strings.HasPrefix(raw, ":") {
		return "http://localhost" + raw
	}
	return "https://" + raw
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
