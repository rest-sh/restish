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
	"Authorization":       true,
	"Cookie":              true,
	"Proxy-Authorization": true,
	"Set-Cookie":          true,
	"X-Api-Key":           true,
	"X-Api-Token":         true,
	"X-Auth-Token":        true,
	"X-Secret":            true,
}

// QueryParamNames contains lower-case query parameter names that commonly
// carry credentials. Matching is exact after strings.ToLower.
var QueryParamNames = map[string]bool{
	"access_token":  true,
	"refresh_token": true,
	"token":         true,
	"api_key":       true,
	"apikey":        true,
	"client_secret": true,
	"password":      true,
	"secret":        true,
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

func IsQueryParamName(name string) bool {
	return QueryParamNames[strings.ToLower(name)]
}

func IsJSONBodyKey(name string) bool {
	return JSONBodyKeys[strings.ToLower(name)]
}

func IsOAuthErrorBodyKey(name string) bool {
	return OAuthErrorBodyKeys[strings.ToLower(name)]
}
