package spec

import "encoding/json"

// XCLIConfig is the x-cli-config extension at the OpenAPI document root.
// It drives `restish api configure` prompts and pre-populates the config file.
type XCLIConfig struct {
	// Profiles maps profile names to their pre-populated settings.
	Profiles map[string]*XCLIProfile `json:"profiles,omitempty"`
}

// XCLIProfile holds pre-populated configuration for a single API profile.
type XCLIProfile struct {
	Headers []string  `json:"headers,omitempty"`
	Query   []string  `json:"query,omitempty"`
	Auth    *XCLIAuth `json:"auth,omitempty"`
}

// XCLIAuth holds pre-populated authentication configuration.
type XCLIAuth struct {
	// Type is the restish auth type (e.g. "bearer", "http-basic").
	Type string `json:"type,omitempty"`
	// Params holds auth parameters; secret values should be empty strings.
	Params map[string]string `json:"params,omitempty"`
}

// ReadXCLIConfig extracts the x-cli-config extension from s.Raw.
// Returns nil, nil when the extension is absent or the spec is not JSON.
func ReadXCLIConfig(s *APISpec) (*XCLIConfig, error) {
	// Parse only the top-level key we care about to avoid full re-parse.
	var top struct {
		XCLIConfig json.RawMessage `json:"x-cli-config"`
	}
	if err := json.Unmarshal(s.Raw, &top); err != nil || top.XCLIConfig == nil {
		return nil, nil
	}
	var cfg XCLIConfig
	if err := json.Unmarshal(top.XCLIConfig, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
