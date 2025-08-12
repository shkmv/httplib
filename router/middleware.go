package router

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "log"
    "net"
    "net/http"
    "runtime/debug"
    "strings"
    "time"
)

// context keys
type contextKey string

const (
    ctxKeyReqID  contextKey = "router_req_id"
    ctxKeyRealIP contextKey = "router_real_ip"
)

// GetReqID returns the request ID from context if present.
func GetReqID(ctx context.Context) string {
    if v := ctx.Value(ctxKeyReqID); v != nil {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return ""
}

// GetRealIP returns the resolved client IP from context if present.
func GetRealIP(ctx context.Context) string {
    if v := ctx.Value(ctxKeyRealIP); v != nil {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return ""
}

// RequestID adds/propagates an X-Request-ID header and stores it in context.
func RequestID() Middleware {
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
            r = r.WithContext(context.WithValue(r.Context(), ctxKeyReqID, id))
            next.ServeHTTP(w, r)
        })
    }
}

// RealIP resolves the client IP using X-Forwarded-For or X-Real-IP and
// stores it in context; it also updates req.RemoteAddr to the resolved IP.
func RealIP() Middleware {
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
            r = r.WithContext(context.WithValue(r.Context(), ctxKeyRealIP, ip))
            next.ServeHTTP(w, r)
        })
    }
}

func realIPFromRequest(r *http.Request) string {
    // X-Forwarded-For: client, proxy1, proxy2
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

// Logger logs basic request information including status, bytes, duration,
// method, path, remote IP, and request ID. If l is nil, log.Default() is used.
func Logger(l *log.Logger) Middleware {
    if l == nil {
        l = log.Default()
    }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            srw := &statusResponseWriter{ResponseWriter: w, status: 0}
            next.ServeHTTP(srw, r)
            dur := time.Since(start)
            ip := GetRealIP(r.Context())
            if ip == "" {
                ip, _, _ = net.SplitHostPort(r.RemoteAddr)
                if ip == "" {
                    ip = r.RemoteAddr
                }
            }
            rid := GetReqID(r.Context())
            if srw.status == 0 {
                srw.status = http.StatusOK
            }
            l.Printf("%s %s %d %dB %s ip=%s req_id=%s", r.Method, r.URL.Path, srw.status, srw.bytes, dur.Truncate(time.Microsecond), ip, rid)
        })
    }
}

// Recoverer recovers from panics, logs a stack trace, and returns 500.
func Recoverer(l *log.Logger) Middleware {
    if l == nil {
        l = log.Default()
    }
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

// Timeout sets a deadline for the request using the stdlib TimeoutHandler.
// If the timeout is exceeded, a 503 is returned with the provided message.
func Timeout(d time.Duration, msg string) Middleware {
    if msg == "" {
        msg = "request timeout"
    }
    return func(next http.Handler) http.Handler { return http.TimeoutHandler(next, d, msg) }
}

// NoCache sets response headers to prevent caching.
func NoCache() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0, private")
            w.Header().Set("Pragma", "no-cache")
            w.Header().Set("Expires", "0")
            next.ServeHTTP(w, r)
        })
    }
}

// statusResponseWriter captures status code and bytes written.
type statusResponseWriter struct {
    http.ResponseWriter
    status int
    bytes  int
}

func (w *statusResponseWriter) WriteHeader(code int) {
    w.status = code
    w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
    if w.status == 0 {
        w.status = http.StatusOK
    }
    n, err := w.ResponseWriter.Write(b)
    w.bytes += n
    return n, err
}

