package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIContentTypes(t *testing.T) {
	captured := run("api content-types")
	assert.Contains(t, captured, "application/json")
	assert.Contains(t, captured, "table")
	assert.Contains(t, captured, "readable")
}

func TestAPIShow(t *testing.T) {
	for tn, tc := range map[string]struct {
		color bool
		want  string
	}{
		"no color": {false, "{\n  \"base\": \"https://api.example.com\"\n}\n"},
		"color":    {true, "\x1b[38;5;247m{\x1b[0m\n  \x1b[38;5;74m\"base\"\x1b[0m\x1b[38;5;247m:\x1b[0m \x1b[38;5;150m\"https://api.example.com\"\x1b[0m\n\x1b[38;5;247m}\x1b[0m\n"},
	} {
		t.Run(tn, func(t *testing.T) {
			reset(tc.color)
			configs["test"] = &APIConfig{
				name: "test",
				Base: "https://api.example.com",
			}
			captured := runNoReset("api show test")
			assert.Equal(t, captured, tc.want)
		})
	}
}

func TestAPIClearCache(t *testing.T) {
	reset(false)

	configs["test"] = &APIConfig{
		name: "test",
		Base: "https://api.example.com",
	}
	Cache.Set("test:default.token", "abc123")

	runNoReset("api clear-auth-cache test")

	assert.Equal(t, "", Cache.GetString("test:default.token"))
}

func TestAPIClearCacheProfile(t *testing.T) {
	reset(false)

	configs["test"] = &APIConfig{
		name: "test",
		Base: "https://api.example.com",
	}
	Cache.Set("test:default.token", "abc123")
	Cache.Set("test:other.token", "def456")

	runNoReset("api clear-auth-cache test -p other")

	assert.Equal(t, "abc123", Cache.GetString("test:default.token"))
	assert.Equal(t, "", Cache.GetString("test:other.token"))
}

func TestAPIClearCacheMissing(t *testing.T) {
	reset(false)

	captured := runNoReset("api clear-auth-cache missing-api")
	assert.Contains(t, captured, "API missing-api not found")
}

func TestEditAPIsMissingEditor(t *testing.T) {
	os.Setenv("EDITOR", "")
	os.Setenv("VISUAL", "")
	exited := false
	editAPIs(func(code int) {
		exited = true
	})
	assert.True(t, exited)
}

func TestEditBadCommand(t *testing.T) {
	os.Setenv("EDITOR", "bad-command")
	os.Setenv("VISUAL", "")
	assert.Panics(t, func() {
		editAPIs(func(code int) {})
	})
}

func TestFindLocalConfig(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Test case 1: No local config found
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	os.Chdir(tmpDir)
	config := findLocalConfig()
	assert.Equal(t, "", config)

	// Test case 2: .restish.json in current directory
	configPath := tmpDir + "/.restish.json"
	os.WriteFile(configPath, []byte(`{"test-api": {"base": "https://test.example.com"}}`), 0644)
	config = findLocalConfig()
	assert.Equal(t, configPath, config)

	// Test case 3: .restish.yaml in current directory (should prefer .json)
	yamlPath := tmpDir + "/.restish.yaml"
	os.WriteFile(yamlPath, []byte(`test-api:\n  base: https://test.example.com`), 0644)
	config = findLocalConfig()
	assert.Equal(t, configPath, config)

	// Test case 4: Only .restish.yaml exists
	os.Remove(configPath)
	config = findLocalConfig()
	assert.Equal(t, yamlPath, config)

	// Test case 5: Config in parent directory
	subDir := tmpDir + "/subdir"
	os.Mkdir(subDir, 0755)
	os.Chdir(subDir)
	config = findLocalConfig()
	assert.Equal(t, yamlPath, config)
}

func TestLoadLocalConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := tmpDir + "/.restish.json"

	// Write a local config with relative spec_files path
	localConfig := `{
		"local-api": {
			"base": "https://local.example.com",
			"spec_files": ["openapi.yaml", "/absolute/path.yaml", "http://example.com/spec.yaml"]
		}
	}`
	os.WriteFile(configPath, []byte(localConfig), 0644)

	// Reset to ensure clean state
	reset(false)

	// Load the local config
	err := loadLocalConfig(configPath)
	assert.NoError(t, err)

	// Verify the config was loaded
	var loadedConfig APIConfig
	apis.UnmarshalKey("local-api", &loadedConfig)
	assert.Equal(t, "https://local.example.com", loadedConfig.Base)
	assert.Len(t, loadedConfig.SpecFiles, 3)

	// Check that relative path was resolved
	assert.Equal(t, tmpDir+"/openapi.yaml", loadedConfig.SpecFiles[0])
	// Check that absolute path was not modified
	assert.Equal(t, "/absolute/path.yaml", loadedConfig.SpecFiles[1])
	// Check that URL was not modified
	assert.Equal(t, "http://example.com/spec.yaml", loadedConfig.SpecFiles[2])
}
