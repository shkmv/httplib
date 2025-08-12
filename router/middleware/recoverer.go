package middleware

import (
    "log"
    "net/http"
    "runtime/debug"

    "github.com/shkmv/httplib/router"
)

// Recoverer recovers from panics, logs stack, and returns 500.
func Recoverer(l *log.Logger) router.Middleware {
    if l == nil { l = log.Default() }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    l.Printf("panic: %v\n%s", rec, debug.Stack())
                    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}

