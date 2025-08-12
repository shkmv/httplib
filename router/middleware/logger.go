package middleware

import (
    "log"
    "net"
    "net/http"
    "time"

    "github.com/shkmv/httplib/router"
    "github.com/shkmv/httplib/router/ctxutil"
)

// Logger logs method, path, status, bytes, duration, IP, and request ID.
func Logger(l *log.Logger) router.Middleware {
    if l == nil { l = log.Default() }
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            srw := &statusResponseWriter{ResponseWriter: w}
            next.ServeHTTP(srw, r)
            dur := time.Since(start)
            ip := ctxutil.GetRealIP(r.Context())
            if ip == "" { ip, _, _ = net.SplitHostPort(r.RemoteAddr) }
            rid := ctxutil.GetReqID(r.Context())
            if srw.status == 0 { srw.status = http.StatusOK }
            l.Printf("%s %s %d %dB %s ip=%s req_id=%s", r.Method, r.URL.Path, srw.status, srw.bytes, dur.Truncate(time.Microsecond), ip, rid)
        })
    }
}

type statusResponseWriter struct {
    http.ResponseWriter
    status int
    bytes  int
}

func (w *statusResponseWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusResponseWriter) Write(b []byte) (int, error) {
    if w.status == 0 { w.status = http.StatusOK }
    n, err := w.ResponseWriter.Write(b)
    w.bytes += n
    return n, err
}

