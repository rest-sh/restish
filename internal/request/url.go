package request

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Normalize ensures rawURL has a scheme, expanding convenience shorthand:
//
//   - ":<port>/path"      → "http://localhost:<port>/path"
//   - "example.com/items" → "https://example.com/items"
//
// If serverOverride is non-empty (e.g. "https://staging.example.com/v2"),
// the scheme and host of the resulting URL are replaced with those from
// serverOverride. A path on the override is prefixed to the request path.
func Normalize(rawURL, serverOverride string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)

	// Bare port shorthand: ":8080/path" → "http://localhost:8080/path"
	if strings.HasPrefix(rawURL, ":") {
		rawURL = "http://localhost" + rawURL
	}

	// No scheme present: prepend https://
	if !strings.Contains(rawURL, "://") {
		defaultScheme := "https://"
		hostPort := rawURL
		if slash := strings.IndexByte(hostPort, '/'); slash >= 0 {
			hostPort = hostPort[:slash]
		}
		host := hostPort
		if cut := strings.IndexByte(host, ':'); cut >= 0 {
			host = host[:cut]
		}
		if host == "localhost" || strings.HasPrefix(host, "127.") {
			defaultScheme = "http://"
		}
		rawURL = defaultScheme + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	if serverOverride != "" {
		override, err := url.Parse(serverOverride)
		if err != nil {
			return "", fmt.Errorf("invalid --rsh-server %q: %w", serverOverride, err)
		}
		if override.Scheme != "http" && override.Scheme != "https" {
			return "", fmt.Errorf("invalid --rsh-server %q: scheme must be http or https", serverOverride)
		}
		u.Scheme = override.Scheme
		u.Host = override.Host
		if override.Path != "" && override.Path != "/" {
			u.Path = joinURLPath(override.EscapedPath(), u.EscapedPath())
			u.RawPath = ""
		}
	}

	return u.String(), nil
}

func joinURLPath(prefix, requestPath string) string {
	if requestPath == "" || requestPath == "/" {
		return path.Clean("/" + strings.TrimPrefix(prefix, "/"))
	}
	return path.Join("/", prefix, requestPath)
}
