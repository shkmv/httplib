package ctxutil

import (
    "context"
)

type contextKey string

const (
    keyReqID  contextKey = "router_req_id"
    keyRealIP contextKey = "router_real_ip"
)

// WithReqID stores a request ID in the context.
func WithReqID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, keyReqID, id)
}

// WithRealIP stores a resolved client IP in the context.
func WithRealIP(ctx context.Context, ip string) context.Context {
    return context.WithValue(ctx, keyRealIP, ip)
}

// GetReqID retrieves a request ID from the context, if set.
func GetReqID(ctx context.Context) string {
    if v := ctx.Value(keyReqID); v != nil {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return ""
}

// GetRealIP retrieves a resolved client IP from the context, if set.
func GetRealIP(ctx context.Context) string {
    if v := ctx.Value(keyRealIP); v != nil {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return ""
}

