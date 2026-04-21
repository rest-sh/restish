package config

import (
	"errors"
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

func TestPaths_CacheFromXDGCacheHome(t *testing.T) {
	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")
	p := NewPaths()
	if got, want := p.Cache(), "/tmp/xdg-cache/restish"; got != want {
		t.Fatalf("Cache() = %q, want %q", got, want)
	}
}

func TestPaths_WindowsDefaultsFromUserDirs(t *testing.T) {
	oldGOOS := runtimeGOOS
	oldUserConfig := userConfigDirFunc
	oldUserCache := userCacheDirFunc
	runtimeGOOS = "windows"
	userConfigDirFunc = func() (string, error) { return `C:\Users\me\AppData\Roaming`, nil }
	userCacheDirFunc = func() (string, error) { return `C:\Users\me\AppData\Local`, nil }
	t.Setenv("RSH_CONFIG_DIR", "")
	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	defer func() {
		runtimeGOOS = oldGOOS
		userConfigDirFunc = oldUserConfig
		userCacheDirFunc = oldUserCache
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
	runtimeGOOS = "linux"
	userConfigDirFunc = func() (string, error) { return "", errors.New("no dir") }
	userCacheDirFunc = func() (string, error) { return "", errors.New("no dir") }
	t.Setenv("RSH_CONFIG_DIR", "")
	t.Setenv("RSH_CACHE_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	defer func() {
		runtimeGOOS = oldGOOS
		userConfigDirFunc = oldUserConfig
		userCacheDirFunc = oldUserCache
	}()

	p := NewPaths()
	if got, want := p.Config(), ".restish"; got != want {
		t.Fatalf("Config() = %q, want %q", got, want)
	}
	if got, want := p.Cache(), filepath.Join(".restish", "cache"); got != want {
		t.Fatalf("Cache() = %q, want %q", got, want)
	}
}
