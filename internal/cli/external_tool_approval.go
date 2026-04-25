package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rest-sh/restish/v2/internal/config"
)

type externalToolApprovals struct {
	Approved []string `json:"approved"`
}

func (c *CLI) ensureExternalToolApproved(ctx context.Context, apiName, profileName, commandLine string) error {
	if commandLine == "" {
		return nil
	}
	hash := externalToolCommandHash(commandLine)
	approvals, approved, err := c.loadExternalToolApprovals(hash)
	if err != nil {
		return err
	}
	if approved {
		return nil
	}
	ok, err := c.Confirm(ctx, fmt.Sprintf("Approve external auth tool for %s/%s?\n  %s\nCommand SHA-256: %s\nRun this command for auth? [y/N] ", apiName, profileName, commandLine, hash))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("external-tool auth command was not approved")
	}
	approvals[hash] = true
	return c.saveExternalToolApprovals(approvals)
}

func externalToolCommandHash(commandLine string) string {
	sum := sha256.Sum256([]byte(commandLine))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (c *CLI) externalToolApprovalsPath() string {
	return filepath.Join(filepath.Dir(c.configFilePath()), "external-tool-approvals.json")
}

func (c *CLI) loadExternalToolApprovals(hash string) (map[string]bool, bool, error) {
	path := c.externalToolApprovalsPath()
	if insecure, err := config.ConfigFileHasInsecurePermissions(path); err != nil {
		return nil, false, err
	} else if insecure {
		return nil, false, fmt.Errorf("external-tool approvals %s is group/world-readable; run chmod 600 %s", path, path)
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var stored externalToolApprovals
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, false, fmt.Errorf("external-tool approvals: %w", err)
	}
	approvals := make(map[string]bool, len(stored.Approved))
	for _, approvedHash := range stored.Approved {
		approvals[approvedHash] = true
	}
	return approvals, approvals[hash], nil
}

func (c *CLI) saveExternalToolApprovals(approvals map[string]bool) error {
	path := c.externalToolApprovalsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock, err := config.LockSiblingFile(path)
	if err != nil {
		return err
	}
	defer lock.Close()

	list := make([]string, 0, len(approvals))
	for hash := range approvals {
		list = append(list, hash)
	}
	sort.Strings(list)
	data, err := json.MarshalIndent(externalToolApprovals{Approved: list}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
