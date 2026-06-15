package auth

import (
	"context"
	"fmt"
	"net/http"
)

// HTTPBasic implements HTTP Basic authentication (RFC 7617).
type HTTPBasic struct {
	// Prompter is called when "password" is absent from params.
	// It receives the prompt string and must return the secret value.
	// If nil, a missing password causes an error.
	Prompter func(prompt string) (string, error)
}

func (h *HTTPBasic) Parameters() []Param {
	return []Param{
		{Name: "username", Description: "HTTP Basic auth username", Required: true},
		{Name: "password", Description: "HTTP Basic auth password (prompted if omitted)", Required: false, Secret: true},
	}
}

func (h *HTTPBasic) OnRequest(req *http.Request, params map[string]string) error {
	user := params["username"]
	if user == "" {
		return fmt.Errorf("http-basic: username is required")
	}
	pass := params["password"]
	if pass == "" {
		if h.Prompter == nil {
			return fmt.Errorf("http-basic: password is required (no prompter configured)")
		}
		var err error
		pass, err = h.Prompter("Password: ")
		if err != nil {
			return fmt.Errorf("http-basic: prompting for password: %w", err)
		}
	}
	if getHeaderCaseInsensitive(req.Header, "Authorization") == "" {
		req.SetBasicAuth(user, pass)
	}
	return nil
}

func (h *HTTPBasic) Authenticate(_ context.Context, req *http.Request, ac AuthContext) error {
	prompter := h.Prompter
	if prompter == nil && ac.Prompter != nil {
		prompter = ac.Prompter.PromptSecret
	}
	return (&HTTPBasic{Prompter: prompter}).OnRequest(req, ac.Params)
}
