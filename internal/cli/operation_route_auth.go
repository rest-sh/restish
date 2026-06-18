package cli

import (
	"context"
	"net/url"
	urlpath "path"
	"strings"

	"github.com/rest-sh/restish/v2/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

type genericOperationAuthMatch struct {
	policy *operationAuthPolicy
	noAuth bool
}

func (c *CLI) operationAuthForGenericRequest(ctx context.Context, method, rawURL, apiName, profileName string) (genericOperationAuthMatch, bool) {
	if method == "" || c.cfg == nil || c.cfg.APIs == nil {
		return genericOperationAuthMatch{}, false
	}
	apiCfg := c.cfg.APIs[apiName]
	if apiCfg == nil {
		return genericOperationAuthMatch{}, false
	}
	set, ok := c.cachedOperationSetForAPI(ctx, apiName, apiCfg, profileName)
	if !ok || len(set.Operations) == 0 {
		return genericOperationAuthMatch{}, false
	}
	requestPath, ok := requestURLPath(rawURL)
	if !ok {
		return genericOperationAuthMatch{}, false
	}

	method = strings.ToUpper(method)
	var best spec.Operation
	bestScore := -1
	ambiguous := false
	for _, op := range set.Operations {
		if !strings.EqualFold(op.Method, method) {
			continue
		}
		routePath, ok := operationRoutePath(apiCfg, profileName, op.Path)
		if !ok {
			continue
		}
		score, ok := routeTemplateMatchScore(routePath, requestPath)
		if !ok || score < bestScore {
			continue
		}
		if score == bestScore {
			ambiguous = true
			continue
		}
		best = op
		bestScore = score
		ambiguous = false
	}
	if bestScore < 0 || ambiguous {
		return genericOperationAuthMatch{}, false
	}
	if best.NoAuth {
		return genericOperationAuthMatch{noAuth: true}, true
	}
	if len(best.CredentialAlternatives) == 0 && !best.OptionalAuth {
		return genericOperationAuthMatch{}, false
	}
	return genericOperationAuthMatch{
		policy: &operationAuthPolicy{
			OptionalAuth:           best.OptionalAuth,
			CredentialAlternatives: best.CredentialAlternatives,
		},
	}, true
}

func requestURLPath(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || !u.IsAbs() {
		return "", false
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	return urlpath.Clean(path), true
}

func operationRoutePath(apiCfg *config.APIConfig, profileName, opPath string) (string, bool) {
	baseURL := effectiveProfileBaseURL(apiCfg, profileName)
	operationBase := effectiveOperationBase(apiCfg, profileName)
	if baseURL == "" {
		return "", false
	}
	var raw string
	if operationBase != "" {
		resolved, err := config.ResolveOperationBaseURL(baseURL, operationBase)
		if err != nil {
			return "", false
		}
		raw = strings.TrimRight(resolved, "/") + opPath
	} else {
		raw = strings.TrimRight(baseURL, "/") + opPath
	}
	u, err := url.Parse(cleanExpandedAPIURL(raw))
	if err != nil {
		return "", false
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	return urlpath.Clean(path), true
}

func routeTemplateMatchScore(templatePath, requestPath string) (int, bool) {
	templatePath = urlpath.Clean(templatePath)
	requestPath = urlpath.Clean(requestPath)
	if templatePath == requestPath && !strings.Contains(templatePath, "{") {
		return len(templatePath) * 4, true
	}
	templateSegments := splitCleanPath(templatePath)
	requestSegments := splitCleanPath(requestPath)
	if len(templateSegments) != len(requestSegments) {
		return 0, false
	}
	score := 0
	for i, templateSegment := range templateSegments {
		requestSegment := requestSegments[i]
		if strings.Contains(templateSegment, "{") && strings.Contains(templateSegment, "}") {
			if requestSegment == "" {
				return 0, false
			}
			score += literalTemplateChars(templateSegment)
			continue
		}
		if templateSegment != requestSegment {
			return 0, false
		}
		score += len(templateSegment) * 4
	}
	return score, true
}

func splitCleanPath(p string) []string {
	p = strings.Trim(urlpath.Clean(p), "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func literalTemplateChars(segment string) int {
	score := 0
	inTemplate := false
	for _, r := range segment {
		switch r {
		case '{':
			inTemplate = true
		case '}':
			inTemplate = false
		default:
			if !inTemplate {
				score++
			}
		}
	}
	return score
}
