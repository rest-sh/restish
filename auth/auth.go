// Package auth provides the public Handler interface for custom authentication
// schemes in embedded Restish CLI instances.
//
// Implement Handler to add a custom auth strategy (e.g. AWS SigV4, HMAC):
//
//	type MySigV4 struct{}
//	func (h *MySigV4) Parameters() []auth.Param { ... }
//	func (h *MySigV4) Authenticate(ctx context.Context, req *http.Request, ac auth.AuthContext) error { ... }
//
// Register it with CLI.AddAuthHandler before calling CLI.Run:
//
//	cli.AddAuthHandler("aws-sigv4", &MySigV4{})
package auth

import internalauth "github.com/danielgtaylor/restish/v2/internal/auth"

// Param describes a single configuration parameter required by an auth Handler.
// Secret parameters are redacted in output and excluded from plugin dispatch
// unless the plugin explicitly opts in with needs_auth_secrets in its manifest.
type Param = internalauth.Param
type AuthContext = internalauth.AuthContext
type TokenStore = internalauth.TokenStore
type Prompter = internalauth.Prompter
type Logger = internalauth.Logger

// Handler is the interface implemented by each auth mechanism.
// The built-in types (http-basic, oauth-client-credentials,
// oauth-authorization-code, oauth-device-code, external-tool) already implement this interface.
// Embed or implement Handler to add custom auth schemes.
type Handler = internalauth.Handler
