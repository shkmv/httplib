package middleware_test

import (
    "bytes"
    "io"
    "log"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    "github.com/shkmv/httplib/router"
    "github.com/shkmv/httplib/router/ctxutil"
    mw "github.com/shkmv/httplib/router/middleware"
)

func TestRequestID(t *testing.T) {
    r := router.New()
    r.Use(mw.RequestID())
    r.GetFunc("/id", func(w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, ctxutil.GetReqID(req.Context()))
    })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/id", nil))
    if rr.Code != 200 {
        t.Fatalf("unexpected status: %d", rr.Code)
    }
    idHeader := rr.Header().Get("X-Request-ID")
    if idHeader == "" || rr.Body.String() == "" || rr.Body.String() != idHeader {
        t.Fatalf("request id missing or mismatched header/body: hdr=%q body=%q", idHeader, rr.Body.String())
    }
}

func TestRealIP(t *testing.T) {
    r := router.New()
    r.Use(mw.RealIP())
    r.GetFunc("/ip", func(w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, ctxutil.GetRealIP(req.Context()))
    })

    req := httptest.NewRequest(http.MethodGet, "/ip", nil)
    req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    if got := strings.TrimSpace(rr.Body.String()); got != "1.2.3.4" {
        t.Fatalf("unexpected real ip: %q", got)
    }
}

func TestRecoverer(t *testing.T) {
    r := router.New()
    r.Use(mw.Recoverer(nil))
    r.GetFunc("/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/panic", nil))
    if rr.Code != http.StatusInternalServerError {
        t.Fatalf("expected 500, got %d", rr.Code)
    }
}

func TestTimeout(t *testing.T) {
    r := router.New()
    r.Use(mw.Timeout(10*time.Millisecond, "request timeout"))
    r.GetFunc("/slow", func(w http.ResponseWriter, req *http.Request) {
        time.Sleep(50 * time.Millisecond)
        io.WriteString(w, "done")
    })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/slow", nil))
    if rr.Code != http.StatusServiceUnavailable {
        t.Fatalf("expected 503, got %d", rr.Code)
    }
    if !strings.Contains(rr.Body.String(), "request timeout") {
        t.Fatalf("expected timeout message, got %q", rr.Body.String())
    }
}

func TestNoCache(t *testing.T) {
    r := router.New()
    r.Use(mw.NoCache())
    r.GetFunc("/x", func(w http.ResponseWriter, req *http.Request) { io.WriteString(w, "ok") })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
    if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
        t.Fatalf("expected no-cache, got %q", cc)
    }
}

func TestLogger(t *testing.T) {
    var buf bytes.Buffer
    l := log.New(&buf, "", 0)

    r := router.New()
    r.Use(mw.RealIP()) // ensure ip present
    r.Use(mw.RequestID())
    r.Use(mw.Logger(l))
    r.GetFunc("/x", func(w http.ResponseWriter, req *http.Request) { io.WriteString(w, "ok") })

    req := httptest.NewRequest(http.MethodGet, "/x", nil)
    req.Header.Set("X-Real-IP", "9.8.7.6")
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)

    out := buf.String()
    if !strings.Contains(out, "GET /x 200") || !strings.Contains(out, "ip=9.8.7.6") || !strings.Contains(out, "req_id=") {
        t.Fatalf("unexpected log line: %q", out)
    }
}

func TestCORSPreflight(t *testing.T) {
    r := router.New()
    r.Use(mw.CORS())
    r.GetFunc("/x", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })

    req := httptest.NewRequest(http.MethodOptions, "/x", nil)
    req.Header.Set("Origin", "https://example.com")
    req.Header.Set("Access-Control-Request-Method", "GET")
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusNoContent {
        t.Fatalf("expected 204, got %d", rr.Code)
    }
    if rr.Header().Get("Access-Control-Allow-Origin") == "" {
        t.Fatalf("missing ACAO header")
    }
}

func TestCORSActual(t *testing.T) {
    r := router.New()
    r.Use(mw.CORS(mw.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}, AllowCredentials: true}))
    r.GetFunc("/x", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })

    req := httptest.NewRequest(http.MethodGet, "/x", nil)
    req.Header.Set("Origin", "https://app.example.com")
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    if rr.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
        t.Fatalf("expected echo origin, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
    }
}

