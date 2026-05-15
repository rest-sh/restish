package auth

import (
	"context"
	"net/http"
)

func requestWithContext(req *http.Request, ctx context.Context) *http.Request {
	if req == nil || ctx == nil || req.Context() == ctx {
		return req
	}
	return req.WithContext(ctx)
}
