package request

import (
	"fmt"
	"net/url"
	"strings"
)

// Normalize ensures rawURL has a scheme, expanding convenience shorthand:
//
//   - ":<port>/path"      → "http://localhost:<port>/path"
//   - "example.com/items" → "https://example.com/items"
//
// If serverOverride is non-empty (e.g. "https://staging.example.com"),
// the scheme and host of the resulting URL are replaced with those from
// serverOverride, leaving the path and query string intact.
func Normalize(rawURL, serverOverride string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)

	// Bare port shorthand: ":8080/path" → "http://localhost:8080/path"
	if strings.HasPrefix(rawURL, ":") {
		rawURL = "http://localhost" + rawURL
	}

	// No scheme present: prepend https://
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
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
		u.Scheme = override.Scheme
		u.Host = override.Host
	}

	return u.String(), nil
}
