// Package secrets centralizes the small allow-lists Restish uses to recognize
// credential-bearing names at trust boundaries.
package secrets

import (
	"net/http"
	"strings"
)

// HeaderNames contains canonical HTTP header names that commonly carry
// credentials. Matching is exact after http.CanonicalHeaderKey.
var HeaderNames = map[string]bool{
	"Authorization":             true,
	"Cookie":                    true,
	"Api-Key":                   true,
	"Ocp-Apim-Subscription-Key": true,
	"Proxy-Authorization":       true,
	"Set-Cookie":                true,
	"X-Api-Key":                 true,
	"X-Api-Token":               true,
	"X-Auth-Token":              true,
	"X-Secret":                  true,
}

// QueryParamNames contains lower-case query parameter names that commonly
// carry credentials. Matching is exact after strings.ToLower.
var QueryParamNames = map[string]bool{
	"access_token":     true,
	"refresh_token":    true,
	"id_token":         true,
	"token":            true,
	"api_key":          true,
	"apikey":           true,
	"client_secret":    true,
	"password":         true,
	"secret":           true,
	"subscription-key": true,
}

var ambiguousQueryParamNames = map[string]bool{
	"auth": true,
	"key":  true,
}

// JSONBodyKeys contains lower-case JSON object keys that should be redacted in
// verbose body logging.
var JSONBodyKeys = map[string]bool{
	"access_token":        true,
	"refresh_token":       true,
	"id_token":            true,
	"token":               true,
	"api_key":             true,
	"apikey":              true,
	"client_secret":       true,
	"password":            true,
	"secret":              true,
	"assertion":           true,
	"authorization":       true,
	"cookie":              true,
	"proxy-authorization": true,
	"set-cookie":          true,
}

// OAuthErrorBodyKeys contains lower-case token endpoint error JSON keys that
// should be redacted before surfacing the response body. It intentionally does
// not include token_type.
var OAuthErrorBodyKeys = map[string]bool{
	"access_token":  true,
	"refresh_token": true,
	"id_token":      true,
	"token":         true,
	"client_secret": true,
	"password":      true,
	"assertion":     true,
}

func IsHeaderName(name string) bool {
	return HeaderNames[http.CanonicalHeaderKey(name)]
}

func IsHeaderValue(name, value string) bool {
	if IsHeaderName(name) {
		return true
	}
	name = http.CanonicalHeaderKey(name)
	return strings.HasSuffix(name, "-Key") && LooksSensitiveValue(value)
}

func IsQueryParamName(name string) bool {
	return QueryParamNames[strings.ToLower(name)]
}

func IsQueryParamValue(name, value string) bool {
	name = strings.ToLower(name)
	if QueryParamNames[name] {
		return true
	}
	return ambiguousQueryParamNames[name] && LooksSensitiveValue(value)
}

func IsJSONBodyKey(name string) bool {
	return JSONBodyKeys[strings.ToLower(name)]
}

func IsJSONBodyValue(name, value string) bool {
	name = strings.ToLower(name)
	if JSONBodyKeys[name] || IsHeaderName(name) {
		return true
	}
	return ambiguousQueryParamNames[name] && LooksSensitiveValue(value)
}

func IsOAuthErrorBodyKey(name string) bool {
	return OAuthErrorBodyKeys[strings.ToLower(name)]
}

// LooksSensitiveValue reports whether a value resembles an API key or token.
// It intentionally keeps common low-entropy values such as "testing" visible
// so ambiguous names like "key" can still be useful in verbose diagnostics.
func LooksSensitiveValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	switch lower {
	case "test", "testing", "example", "sample", "demo", "none", "null", "true", "false", "dev", "prod", "stage", "local", "localhost":
		return false
	}
	if strings.HasPrefix(lower, "bearer ") {
		value = strings.TrimSpace(value[len("bearer "):])
	} else if strings.HasPrefix(lower, "basic ") {
		value = strings.TrimSpace(value[len("basic "):])
	}
	if len(value) < 7 {
		return false
	}
	var hasLetter, hasDigit, hasSymbol bool
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '_' || r == '-' || r == '.' || r == '~' || r == '+' || r == '/' || r == '=':
			hasSymbol = true
		default:
			return false
		}
	}
	if hasLetter && hasDigit {
		return true
	}
	if hasSymbol && len(value) >= 10 {
		return true
	}
	return len(value) >= 20 && (hasLetter || hasDigit)
}
