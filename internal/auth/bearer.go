package auth

import (
	"context"
	"fmt"
	"net/http"
)

// Bearer implements static HTTP Bearer token authentication.
type Bearer struct{}

func (h *Bearer) Parameters() []Param {
	return []Param{
		{Name: "token", Description: "Bearer token", Required: true, Secret: true},
	}
}

func (h *Bearer) Authenticate(_ context.Context, req *http.Request, ac AuthContext) error {
	token := ac.Params["token"]
	if token == "" {
		return fmt.Errorf("bearer: token is required")
	}
	bearerAuth(req, token)
	return nil
}
