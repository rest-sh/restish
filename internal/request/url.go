package request

import (
	"fmt"
	"net"
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
		host := hostFromURLWithoutScheme(rawURL)
		if host == "localhost" || isLoopbackIP(host) {
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
		if override.Host == "" {
			return "", fmt.Errorf("invalid --rsh-server %q: host is required", serverOverride)
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

func hostFromURLWithoutScheme(rawURL string) string {
	hostPort := rawURL
	if slash := strings.IndexByte(hostPort, '/'); slash >= 0 {
		hostPort = hostPort[:slash]
	}
	if strings.HasPrefix(hostPort, "[") {
		if end := strings.IndexByte(hostPort, ']'); end > 0 {
			return hostPort[1:end]
		}
	}
	if host, _, err := net.SplitHostPort(hostPort); err == nil {
		return strings.Trim(host, "[]")
	}
	if cut := strings.IndexByte(hostPort, ':'); cut >= 0 {
		return hostPort[:cut]
	}
	return hostPort
}

func isLoopbackIP(host string) bool {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func joinURLPath(prefix, requestPath string) string {
	if requestPath == "" || requestPath == "/" {
		return path.Clean("/" + strings.TrimPrefix(prefix, "/"))
	}
	return path.Join("/", prefix, requestPath)
}
