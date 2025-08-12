package middleware

import (
    "net/http"
    "strconv"
    "strings"

    "github.com/shkmv/httplib/router"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
    AllowedOrigins     []string
    AllowedMethods     []string
    AllowedHeaders     []string
    ExposedHeaders     []string
    AllowCredentials   bool
    MaxAge             int // seconds
    AllowOriginFunc    func(origin string) bool
}

func defaultCORSConfig() CORSConfig {
    return CORSConfig{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
        ExposedHeaders:   nil,
        AllowCredentials: false,
        MaxAge:           300,
    }
}

// CORS returns a middleware implementing Cross-Origin Resource Sharing.
func CORS(cfgs ...CORSConfig) router.Middleware {
    cfg := defaultCORSConfig()
    if len(cfgs) > 0 {
        c := cfgs[0]
        if len(c.AllowedOrigins) > 0 { cfg.AllowedOrigins = c.AllowedOrigins }
        if len(c.AllowedMethods) > 0 { cfg.AllowedMethods = c.AllowedMethods }
        if len(c.AllowedHeaders) > 0 { cfg.AllowedHeaders = c.AllowedHeaders }
        if len(c.ExposedHeaders) > 0 { cfg.ExposedHeaders = c.ExposedHeaders }
        if c.MaxAge != 0 { cfg.MaxAge = c.MaxAge }
        cfg.AllowCredentials = c.AllowCredentials
        cfg.AllowOriginFunc = c.AllowOriginFunc
    }

    allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
    allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")
    exposedHeaders := strings.Join(cfg.ExposedHeaders, ", ")

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")
            if origin == "" {
                next.ServeHTTP(w, r)
                return
            }

            // Always vary on Origin + access-control request headers to avoid cache poisoning
            w.Header().Add("Vary", "Origin")
            w.Header().Add("Vary", "Access-Control-Request-Method")
            w.Header().Add("Vary", "Access-Control-Request-Headers")

            if !isOriginAllowed(origin, cfg) {
                // Not allowed; proceed without CORS headers
                next.ServeHTTP(w, r)
                return
            }

            // If credentials are allowed, echo the origin; else wildcard is fine
            if cfg.AllowCredentials {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Access-Control-Allow-Credentials", "true")
            } else {
                // If specific origins configured, echo the origin; otherwise "*"
                if len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*" {
                    w.Header().Set("Access-Control-Allow-Origin", "*")
                } else {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                }
            }

            if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
                // Preflight
                w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
                if allowedHeaders != "" {
                    w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
                } else if reqHdr := r.Header.Get("Access-Control-Request-Headers"); reqHdr != "" {
                    w.Header().Set("Access-Control-Allow-Headers", reqHdr)
                }
                if cfg.MaxAge > 0 {
                    w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
                }
                w.WriteHeader(http.StatusNoContent)
                return
            }

            if exposedHeaders != "" {
                w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
            }
            next.ServeHTTP(w, r)
        })
    }
}

func isOriginAllowed(origin string, cfg CORSConfig) bool {
    if cfg.AllowOriginFunc != nil {
        return cfg.AllowOriginFunc(origin)
    }
    if len(cfg.AllowedOrigins) == 0 { return false }
    if len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*" { return true }
    for _, o := range cfg.AllowedOrigins {
        if o == origin { return true }
    }
    return false
}

