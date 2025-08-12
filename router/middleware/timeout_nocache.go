package middleware

import (
    "net/http"
    "time"

    "github.com/shkmv/httplib/router"
)

// Timeout sets a request timeout using http.TimeoutHandler.
func Timeout(d time.Duration, msg string) router.Middleware {
    if msg == "" { msg = "request timeout" }
    return func(next http.Handler) http.Handler { return http.TimeoutHandler(next, d, msg) }
}

// NoCache sets headers to disable caching.
func NoCache() router.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0, private")
            w.Header().Set("Pragma", "no-cache")
            w.Header().Set("Expires", "0")
            next.ServeHTTP(w, r)
        })
    }
}

