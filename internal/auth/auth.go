// Package auth is the restish CLI's internal auth-handler implementation.
// External embedders should not import this package; see the public
// github.com/rest-sh/restish/v2/auth package for the token cache API
// and the auth-handler interfaces.
package auth

import (
	"maps"
	"net/http"

	"github.com/rest-sh/restish/v2/auth"
)

func bearerAuth(req *http.Request, token string) {
	if getHeaderCaseInsensitive(req.Header, "Authorization") != "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func authParams(ac auth.AuthContext) map[string]string {
	params := make(map[string]string, len(ac.Params)+1)
	maps.Copy(params, ac.Params)
	switch {
	case ac.CacheKey != "":
		params["_cache_key"] = ac.CacheKey
	case ac.APIName != "" || ac.ProfileName != "":
		params["_cache_key"] = ac.APIName + ":" + ac.ProfileName
	}
	if ac.BaseURL != "" {
		params["_base_url"] = ac.BaseURL
	}
	return params
}
