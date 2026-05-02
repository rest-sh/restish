package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPaths_ConfigFromRSHConfigDir(t *testing.T) {
	t.Setenv("RSH_CONFIG_DIR", "/tmp/rsh-cfg")
	t.Setenv("XDG_CONFIG_HOME", "")
	p := NewPaths()
	if got, want := p.Config(), "/tmp/rsh-cfg"; got != want {
		t.Fatalf("Config() = %q, want %q", got, want)
	}
	if got, want := p.ConfigFile(), filepath.Join("/tmp/rsh-cfg", "restish.json"); got != want {
		t.Fatalf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestPaths_ConfigFromRSHConfigFile(t *testing.T) {
	t.Setenv("RSH_CONFIG", "/tmp/project/restish.json")
	t.Setenv("RSH_CONFIG_DIR", "/tmp/rsh-cfg")
	p := NewPaths()
	if got, want := p.Config(), "/tmp/project"; got != want {
		t.Fatalf("Config() = %q, want %q", got, want)
	}
	if got, want := p.ConfigFile(), "/tmp/project/restish.json"; got != want {
		t.Fatalf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestPaths_WithExplicitConfigFile(t *testing.T) {
	p := NewPathsWithConfigFile("/tmp/work/restish.json")
	if got, want := p.Config(), "/tmp/work"; got != want {
		t.Fatalf("Config() = %q, want %q", got, want)
	}
	if got, want := p.ConfigFile(), "/tmp/work/restish.json"; got != want {
		t.Fatalf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestPaths_CacheFromXDGCacheHome(t *testing.T) {
	oldUserCache := userCacheDirFunc
	userCacheDirFunc = func() (string, error) { return "/tmp/xdg-cache", nil }
	defer func() { userCacheDirFunc = oldUserCache }()

	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")
	p := NewPaths()
	if got, want := p.Cache(), "/tmp/xdg-cache/restish"; got != want {
		t.Fatalf("Cache() = %q, want %q", got, want)
	}
}

func TestPaths_IgnoresRelativeXDGDirs(t *testing.T) {
	oldGOOS := runtimeGOOS
	oldUserHome := userHomeDirFunc
	runtimeGOOS = "darwin"
	userHomeDirFunc = func() (string, error) { return "/Users/me", nil }
	t.Setenv("RSH_CONFIG_DIR", "")
	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "relative-config")
	t.Setenv("XDG_CACHE_HOME", "relative-cache")
	defer func() {
		runtimeGOOS = oldGOOS
		userHomeDirFunc = oldUserHome
	}()

	p := NewPaths()
	if got, want := p.Config(), filepath.Join("/Users/me", ".config", "restish"); got != want {
		t.Fatalf("Config() = %q, want %q", got, want)
	}
	if got, want := p.Cache(), filepath.Join("/Users/me", ".cache", "restish"); got != want {
		t.Fatalf("Cache() = %q, want %q", got, want)
	}
}

func TestPaths_UnixDefaultsUseDotConfigAndDotCache(t *testing.T) {
	oldGOOS := runtimeGOOS
	oldUserConfig := userConfigDirFunc
	oldUserCache := userCacheDirFunc
	oldUserHome := userHomeDirFunc
	runtimeGOOS = "darwin"
	userConfigDirFunc = func() (string, error) { return "/Users/me/Library/Application Support", nil }
	userCacheDirFunc = func() (string, error) { return "/Users/me/Library/Caches", nil }
	userHomeDirFunc = func() (string, error) { return "/Users/me", nil }
	t.Setenv("RSH_CONFIG_DIR", "")
	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	defer func() {
		runtimeGOOS = oldGOOS
		userConfigDirFunc = oldUserConfig
		userCacheDirFunc = oldUserCache
		userHomeDirFunc = oldUserHome
	}()

	p := NewPaths()
	if got, want := p.Config(), filepath.Join("/Users/me", ".config", "restish"); got != want {
		t.Fatalf("Config() = %q, want %q", got, want)
	}
	if got, want := p.Cache(), filepath.Join("/Users/me", ".cache", "restish"); got != want {
		t.Fatalf("Cache() = %q, want %q", got, want)
	}
}

func TestPaths_WindowsDefaultsFromUserDirs(t *testing.T) {
	oldGOOS := runtimeGOOS
	oldUserConfig := userConfigDirFunc
	oldUserCache := userCacheDirFunc
	oldUserHome := userHomeDirFunc
	runtimeGOOS = "windows"
	userConfigDirFunc = func() (string, error) { return `C:\Users\me\AppData\Roaming`, nil }
	userCacheDirFunc = func() (string, error) { return `C:\Users\me\AppData\Local`, nil }
	userHomeDirFunc = func() (string, error) { return `C:\Users\me`, nil }
	t.Setenv("RSH_CONFIG_DIR", "")
	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	defer func() {
		runtimeGOOS = oldGOOS
		userConfigDirFunc = oldUserConfig
		userCacheDirFunc = oldUserCache
		userHomeDirFunc = oldUserHome
	}()

	p := NewPaths()
	if got, want := p.Config(), filepath.Join(`C:\Users\me\AppData\Roaming`, "restish"); got != want {
		t.Fatalf("Config() = %q, want %q", got, want)
	}
	if got, want := p.Cache(), filepath.Join(`C:\Users\me\AppData\Local`, "restish"); got != want {
		t.Fatalf("Cache() = %q, want %q", got, want)
	}
}

func TestPaths_FallbackWhenUserDirsFail(t *testing.T) {
	oldGOOS := runtimeGOOS
	oldUserConfig := userConfigDirFunc
	oldUserCache := userCacheDirFunc
	oldUserHome := userHomeDirFunc
	runtimeGOOS = "darwin"
	userConfigDirFunc = func() (string, error) { return "", errors.New("no dir") }
	userCacheDirFunc = func() (string, error) { return "", errors.New("no dir") }
	userHomeDirFunc = func() (string, error) { return "", errors.New("no home") }
	t.Setenv("RSH_CONFIG_DIR", "")
	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	defer func() {
		runtimeGOOS = oldGOOS
		userConfigDirFunc = oldUserConfig
		userCacheDirFunc = oldUserCache
		userHomeDirFunc = oldUserHome
	}()

	p := NewPaths()
	if got := p.Config(); got != "" {
		t.Fatalf("Config() = %q, want empty when config root cannot be determined", got)
	}
	if err := p.ConfigError(); err == nil {
		t.Fatalf("expected config path error, got %v", err)
	}
	if got, want := p.Cache(), filepath.Join(os.TempDir(), "restish"); got != want {
		t.Fatalf("Cache() = %q, want %q", got, want)
	}
	if err := p.CacheError(); err == nil {
		t.Fatal("expected cache fallback warning error")
	}
}

func TestPaths_ExplicitOverridesAvoidNoHomeErrors(t *testing.T) {
	oldGOOS := runtimeGOOS
	oldUserConfig := userConfigDirFunc
	oldUserCache := userCacheDirFunc
	oldUserHome := userHomeDirFunc
	runtimeGOOS = "darwin"
	userConfigDirFunc = func() (string, error) { return "", errors.New("no dir") }
	userCacheDirFunc = func() (string, error) { return "", errors.New("no dir") }
	userHomeDirFunc = func() (string, error) { return "", errors.New("no home") }
	t.Setenv("RSH_CONFIG_DIR", "/tmp/restish-config")
	t.Setenv("RSH_CACHE_DIR", "/tmp/restish-cache")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	defer func() {
		runtimeGOOS = oldGOOS
		userConfigDirFunc = oldUserConfig
		userCacheDirFunc = oldUserCache
		userHomeDirFunc = oldUserHome
	}()

	p := NewPaths()
	if err := p.ConfigError(); err != nil {
		t.Fatalf("ConfigError() = %v, want nil", err)
	}
	if err := p.CacheError(); err != nil {
		t.Fatalf("CacheError() = %v, want nil", err)
	}
	if got := p.Config(); got != "/tmp/restish-config" {
		t.Fatalf("Config() = %q", got)
	}
	if got := p.Cache(); got != "/tmp/restish-cache" {
		t.Fatalf("Cache() = %q", got)
	}
}
