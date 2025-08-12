package middleware

import (
    "crypto/rand"
    "encoding/hex"
    "net/http"
    "time"

    "github.com/shkmv/httplib/router"
    "github.com/shkmv/httplib/router/ctxutil"
)

// RequestID adds/propagates an X-Request-ID header and stores it in context.
func RequestID() router.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id := r.Header.Get("X-Request-ID")
            if id == "" {
                buf := make([]byte, 16)
                if _, err := rand.Read(buf); err == nil {
                    id = hex.EncodeToString(buf)
                } else {
                    id = time.Now().UTC().Format("20060102T150405.000000000")
                }
            }
            w.Header().Set("X-Request-ID", id)
            r = r.WithContext(ctxutil.WithReqID(r.Context(), id))
            next.ServeHTTP(w, r)
        })
    }
}

