package client

import (
    "bytes"
    "context"
    "encoding/json"
    "io"
    "net/http"
    "sync/atomic"
    "testing"
    "time"
)

// fakeRT is a fake RoundTripper that routes by req.URL.Host and Path.
type fakeRT struct{ handlers map[string]http.Handler }

func (f *fakeRT) RoundTrip(req *http.Request) (*http.Response, error) {
    if h, ok := f.handlers[req.URL.Host]; ok {
        rw := newRespWriter()
        h.ServeHTTP(rw, req)
        if err := req.Context().Err(); err != nil { return nil, err }
        return rw.Result(), nil
    }
    return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(bytes.NewBuffer(nil)), Header: make(http.Header), Request: req}, nil
}

// in-memory ResponseWriter to build http.Response without sockets.
type memRW struct{ header http.Header; code int; buf bytes.Buffer }

func newRespWriter() *memRW { return &memRW{header: make(http.Header), code: 0} }
func (m *memRW) Header() http.Header { return m.header }
func (m *memRW) Write(b []byte) (int, error) { if m.code == 0 { m.code = 200 }; return m.buf.Write(b) }
func (m *memRW) WriteHeader(statusCode int) { m.code = statusCode }
func (m *memRW) Result() *http.Response {
    if m.code == 0 { m.code = 200 }
    return &http.Response{StatusCode: m.code, Header: m.header, Body: io.NopCloser(bytes.NewReader(m.buf.Bytes()))}
}

func TestRoundRobinAcrossEndpoints(t *testing.T) {
    var gotA, gotB int32
    c := New([]Endpoint{{BaseURL: "http://a"}, {BaseURL: "http://b"}})
    c.hc.Transport = &fakeRT{handlers: map[string]http.Handler{
        "a": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&gotA, 1); w.WriteHeader(200) }),
        "b": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&gotB, 1); w.WriteHeader(200) }),
    }}
    for i := 0; i < 10; i++ {
        req, _ := http.NewRequest(http.MethodGet, "/x", nil)
        resp, err := c.Do(context.Background(), req)
        if err != nil { t.Fatalf("do: %v", err) }
        resp.Body.Close()
    }
    if gotA == 0 || gotB == 0 { t.Fatalf("expected traffic to both endpoints: A=%d B=%d", gotA, gotB) }
}

func TestRetryOn500AndFailover(t *testing.T) {
    c := New([]Endpoint{{BaseURL: "http://a"}, {BaseURL: "http://b"}})
    c.retry = DefaultRetryPolicy()
    c.retry.MaxAttempts = 2 // ensure one retry
    c.hc.Transport = &fakeRT{handlers: map[string]http.Handler{
        "a": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }),
        "b": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{"ok": true})
        }),
    }}

    var out struct{ Ok bool `json:"ok"` }
    _, err := c.GetJSON(context.Background(), "/", &out)
    if err != nil { t.Fatalf("get: %v", err) }
    if !out.Ok { t.Fatalf("expected ok true, got %+v", out) }
}

func TestPreferredDC(t *testing.T) {
    var gotPreferred, gotOther int32
    c := New([]Endpoint{{BaseURL: "http://a", DC: "eu"}, {BaseURL: "http://b", DC: "us"}}, WithPreferredDC("eu"))
    c.hc.Transport = &fakeRT{handlers: map[string]http.Handler{
        "a": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&gotPreferred, 1); w.WriteHeader(200) }),
        "b": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&gotOther, 1); w.WriteHeader(200) }),
    }}
    for i := 0; i < 4; i++ {
        req, _ := http.NewRequest(http.MethodGet, "/x", nil)
        resp, err := c.Do(context.Background(), req)
        if err != nil { t.Fatalf("do: %v", err) }
        resp.Body.Close()
    }
    if gotPreferred == 0 { t.Fatalf("expected calls to preferred dc") }
}

func TestContextCancelStopsRetries(t *testing.T) {
    c := New([]Endpoint{{BaseURL: "http://slow"}})
    c.hc.Timeout = 200 * time.Millisecond
    c.retry.MaxAttempts = 5
    c.hc.Transport = &fakeRT{handlers: map[string]http.Handler{
        "slow": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Sleep longer than client timeout
            select {
            case <-time.After(2 * time.Second):
                w.WriteHeader(200)
            case <-r.Context().Done():
                return
            }
        }),
    }}

    ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
    defer cancel()

    req, _ := http.NewRequest(http.MethodGet, "/x", nil)
    _, err := c.Do(ctx, req)
    if err == nil { t.Fatalf("expected error due to timeout") }
}
