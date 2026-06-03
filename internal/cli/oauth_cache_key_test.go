package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

func oauthAuthCodeCacheKeyForTest(params map[string]string) string {
	relevant := map[string]string{"type": "oauth-authorization-code"}
	for key, value := range params {
		if value == "" || oauthAuthCodeCacheKeyIgnoresParamForTest(key) {
			continue
		}
		relevant[key] = value
	}
	keys := make([]string, 0, len(relevant))
	for key := range relevant {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(relevant[key])
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "oauth:" + hex.EncodeToString(sum[:8])
}

func oauthAuthCodeCacheKeyIgnoresParamForTest(key string) bool {
	switch key {
	case "_base_url", "_cache_key", "auth_method", "cache_key", "client_secret",
		"callback_error_html", "callback_success_html",
		"redirect_cert", "redirect_key", "redirect_path", "redirect_port", "redirect_scheme":
		return true
	default:
		return false
	}
}
