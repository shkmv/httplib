package middleware

import (
    "net"
    "net/http"
    "strings"

    "github.com/shkmv/httplib/router"
    "github.com/shkmv/httplib/router/ctxutil"
)

// RealIP resolves the client IP using X-Forwarded-For or X-Real-IP and stores it in context.
func RealIP() router.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := realIPFromRequest(r)
            if ip == "" {
                ip, _, _ = net.SplitHostPort(r.RemoteAddr)
                if ip == "" {
                    ip = r.RemoteAddr
                }
            }
            r.RemoteAddr = ip
            r = r.WithContext(ctxutil.WithRealIP(r.Context(), ip))
            next.ServeHTTP(w, r)
        })
    }
}

func realIPFromRequest(r *http.Request) string {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        parts := strings.Split(xff, ",")
        if len(parts) > 0 {
            s := strings.TrimSpace(parts[0])
            if s != "" {
                return s
            }
        }
    }
    if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
        return rip
    }
    return ""
}

