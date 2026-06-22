package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tidwall/jsonc"
)

// LegacyAPIConfigAuth is the v1 `profiles.<name>.auth` shape.
type LegacyAPIConfigAuth struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params,omitempty"`
}

// LegacyPKCS11Config is the v1 `tls.pkcs11` shape.
type LegacyPKCS11Config struct {
	Path  string `json:"path,omitempty"`
	Label string `json:"label,omitempty"`
}

// LegacyTLSConfig is the v1 `tls` shape.
type LegacyTLSConfig struct {
	PKCS11 *LegacyPKCS11Config `json:"pkcs11,omitempty"`
	Cert   string              `json:"cert,omitempty"`
	Key    string              `json:"key,omitempty"`
	CACert string              `json:"ca_cert,omitempty"`
}

// LegacyAPIProfile is the v1 `profiles.<name>` shape.
type LegacyAPIProfile struct {
	Base    string                  `json:"base,omitempty"`
	Headers map[string]string       `json:"headers,omitempty"`
	Query   map[string]string       `json:"query,omitempty"`
	Auth    *LegacyAPIConfigAuth    `json:"auth,omitempty"`
}

// LegacyAPIConfig is the v1 per-API shape from apis.json.
type LegacyAPIConfig struct {
	Base          string                       `json:"base"`
	OperationBase string                       `json:"operation_base,omitempty"`
	SpecFiles     []string                     `json:"spec_files,omitempty"`
	Profiles      map[string]*LegacyAPIProfile `json:"profiles,omitempty"`
	TLS           *LegacyTLSConfig             `json:"tls,omitempty"`
}

// ConvertLegacyAPI converts a single v1 (apis.json) APIConfig entry to the
// current v2 shape. The name argument is used for warning messages only; it
// does not affect the converted data. Pass the raw JSON value of one entry
// from an apis.json file, e.g. the bytes associated with one key in the
// top-level map.
//
// Returned warnings describe migration-time decisions such as dropped
// invalid operation_base values or migrated PKCS#11 TLS config that now
// requires the restish-pkcs11 plugin to be installed.
//
// The function does not read or write any files. Callers that want a
// one-call read of an apis.json folder can use ReadLegacyAPIs.
func ConvertLegacyAPI(name string, raw json.RawMessage) (*APIConfig, []string, error) {
	stripped := jsonc.ToJSON(raw)
	var legacy LegacyAPIConfig
	if err := json.Unmarshal(stripped, &legacy); err != nil {
		return nil, nil, &ParseError{Path: name, Err: err}
	}
	api, warnings := convertLegacyAPIConfig(name, &legacy)
	return api, warnings, nil
}

// ReadLegacyAPIs reads every API from a v1 apis.json in folder, returning them
// in v2 shape. The "$schema" key is skipped. Returns an empty map and no
// error when apis.json does not exist.
//
// This is the read path for embedders that want to read a legacy config
// without going through the restish CLI's automatic migration. ReadLegacyAPIs
// never modifies the user's apis.json or restish.json; the restish CLI's
// own TryMigrate step is responsible for that.
func ReadLegacyAPIs(folder string) (map[string]*APIConfig, error) {
	path := filepath.Join(folder, "apis.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]*APIConfig{}, nil
		}
		return nil, fmt.Errorf("config: cannot read %s: %w", path, err)
	}
	stripped := jsonc.ToJSON(data)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(stripped, &raw); err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}
	out := make(map[string]*APIConfig, len(raw))
	var warnings []string
	for name, value := range raw {
		if strings.HasPrefix(name, "$") {
			continue
		}
		api, entryWarnings, err := ConvertLegacyAPI(name, value)
		if err != nil {
			return nil, fmt.Errorf("config: parsing %s entry %q: %w", path, name, err)
		}
		out[name] = api
		warnings = append(warnings, entryWarnings...)
	}
	return out, nil
}

// ReadLegacyAPI reads a single named API from a v1 apis.json in folder,
// returning it in v2 shape. Returns an error when the API is not present.
func ReadLegacyAPI(folder, name string) (*APIConfig, error) {
	all, err := ReadLegacyAPIs(folder)
	if err != nil {
		return nil, err
	}
	api, ok := all[name]
	if !ok {
		return nil, fmt.Errorf("config: api %q not found in %s", name, filepath.Join(folder, "apis.json"))
	}
	return api, nil
}

// ReadAPIs returns every API configured in folder, in v2 shape, regardless of
// whether the user's config is in v2 (restish.json) or v1 (apis.json) format.
// This is the format-agnostic read path for embedders.
//
// restish.json is preferred when present. apis.json is read only when
// restish.json does not exist, so a user who has already migrated is served
// from the v2 file with no v1 conversion. ReadAPIs never modifies either
// file; the restish CLI's TryMigrate step is responsible for that.
func ReadAPIs(folder string) (map[string]*APIConfig, error) {
	restishPath := filepath.Join(folder, "restish.json")
	if _, err := os.Stat(restishPath); err == nil {
		cfg, err := Load(restishPath)
		if err != nil {
			return nil, err
		}
		if cfg.APIs == nil {
			return map[string]*APIConfig{}, nil
		}
		return cfg.APIs, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: cannot stat %s: %w", restishPath, err)
	}
	return ReadLegacyAPIs(folder)
}

// convertLegacyAPIConfig converts one parsed v1 entry into the v2 shape and
// emits migration-time warnings. Used by both the public ConvertLegacyAPI
// helper and the internal automatic migrator.
func convertLegacyAPIConfig(name string, legacy *LegacyAPIConfig) (*APIConfig, []string) {
	if legacy == nil {
		return &APIConfig{}, nil
	}
	operationBase, warning := convertLegacyOperationBase(name, legacy.OperationBase)

	api := &APIConfig{
		BaseURL:       legacy.Base,
		OperationBase: operationBase,
		SpecFiles:     append([]string(nil), legacy.SpecFiles...),
	}
	if len(legacy.Profiles) > 0 {
		api.Profiles = make(map[string]*ProfileConfig, len(legacy.Profiles))
		for name, profile := range legacy.Profiles {
			api.Profiles[name] = convertLegacyProfile(profile)
		}
	}

	var migratedPKCS11 bool
	if legacy.TLS != nil && legacy.TLS.PKCS11 != nil {
		if api.Profiles == nil {
			api.Profiles = map[string]*ProfileConfig{}
		}
		prof := api.Profiles["default"]
		if prof == nil {
			prof = &ProfileConfig{}
			api.Profiles["default"] = prof
		}
		if prof.TLSSigner == "" {
			prof.TLSSigner = "pkcs11"
		}
		if prof.TLSSignerParams == nil {
			prof.TLSSignerParams = map[string]string{}
		}
		if legacy.TLS.PKCS11.Path != "" && prof.TLSSignerParams["path"] == "" {
			prof.TLSSignerParams["path"] = legacy.TLS.PKCS11.Path
		}
		if legacy.TLS.PKCS11.Label != "" && prof.TLSSignerParams["label"] == "" {
			prof.TLSSignerParams["label"] = legacy.TLS.PKCS11.Label
		}
		migratedPKCS11 = true
	}

	if legacy.TLS != nil && (legacy.TLS.Cert != "" || legacy.TLS.Key != "") {
		if api.Profiles == nil {
			api.Profiles = map[string]*ProfileConfig{}
		}
		prof := api.Profiles["default"]
		if prof == nil {
			prof = &ProfileConfig{}
			api.Profiles["default"] = prof
		}
		if prof.ClientCertPath == "" && legacy.TLS.Cert != "" {
			prof.ClientCertPath = legacy.TLS.Cert
		}
		if prof.ClientKeyPath == "" && legacy.TLS.Key != "" {
			prof.ClientKeyPath = legacy.TLS.Key
		}
	}

	if legacy.TLS != nil && legacy.TLS.CACert != "" {
		if api.Profiles == nil {
			api.Profiles = map[string]*ProfileConfig{}
		}
		prof := api.Profiles["default"]
		if prof == nil {
			prof = &ProfileConfig{}
			api.Profiles["default"] = prof
		}
		if prof.CACertPath == "" {
			prof.CACertPath = legacy.TLS.CACert
		}
	}

	var warnings []string
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if migratedPKCS11 {
		warnings = append(warnings, fmt.Sprintf("api %q: migrated PKCS#11 TLS config; install the restish-pkcs11 plugin to continue using it (see https://github.com/rest-sh/restish)", name))
	}
	return api, warnings
}

func convertLegacyOperationBase(apiName, operationBase string) (string, string) {
	if operationBase == "" {
		return "", ""
	}
	if err := ValidateOperationBase(operationBase); err == nil {
		return operationBase, ""
	}
	return "", fmt.Sprintf("api %q: dropped invalid legacy operation_base %q; v2 operation_base must be an absolute path", apiName, operationBase)
}

func convertLegacyProfile(legacy *LegacyAPIProfile) *ProfileConfig {
	if legacy == nil {
		return &ProfileConfig{}
	}

	prof := &ProfileConfig{
		BaseURL: legacy.Base,
		Headers: sortedHeaderList(legacy.Headers),
		Query:   sortedQueryList(legacy.Query),
	}
	if legacy.Auth != nil {
		prof.Auth = &AuthConfig{
			Type:   legacy.Auth.Name,
			Params: cloneStringMap(legacy.Auth.Params),
		}
	}
	return prof
}

func sortedHeaderList(values map[string]string) []string {
	return sortedPairs(values, ": ")
}

func sortedQueryList(values map[string]string) []string {
	return sortedPairs(values, "=")
}

func sortedPairs(values map[string]string, sep string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, key+sep+values[key])
	}
	return items
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}
