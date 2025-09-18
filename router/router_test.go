package router

import (
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestRouteGrouping(t *testing.T) {
    r := New()
    r.Route("/api", func(api *Router) {
        api.GetFunc("/ping", func(w http.ResponseWriter, req *http.Request) {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("pong"))
        })
    })

    // Hit grouped route
    req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK || rr.Body.String() != "pong" {
        t.Fatalf("expected 200 pong, got %d %q", rr.Code, rr.Body.String())
    }

    // Non-existent at root
    req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
    rr2 := httptest.NewRecorder()
    r.ServeHTTP(rr2, req2)
    if rr2.Code != http.StatusNotFound {
        t.Fatalf("expected 404 for /ping, got %d", rr2.Code)
    }
}

func TestMethodHandling(t *testing.T) {
    r := New()
    r.PostFunc("/submit", func(w http.ResponseWriter, req *http.Request) {
        w.WriteHeader(http.StatusCreated)
        io.WriteString(w, "ok")
    })

    // Wrong method
    req := httptest.NewRequest(http.MethodGet, "/submit", nil)
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusMethodNotAllowed {
        t.Fatalf("expected 405, got %d", rr.Code)
    }

    // Correct method
    req2 := httptest.NewRequest(http.MethodPost, "/submit", nil)
    rr2 := httptest.NewRecorder()
    r.ServeHTTP(rr2, req2)
    if rr2.Code != http.StatusCreated || rr2.Body.String() != "ok" {
        t.Fatalf("expected 201 ok, got %d %q", rr2.Code, rr2.Body.String())
    }
}

func TestMount(t *testing.T) {
    r := New()
    sub := New()
    sub.GetFunc("/", func(w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, "home")
    })
    sub.GetFunc("/dashboard", func(w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, "dash")
    })

    r.Mount("/admin", sub)

    // Exact mount path should reach sub at "/"
    req := httptest.NewRequest(http.MethodGet, "/admin", nil)
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK || rr.Body.String() != "home" {
        t.Fatalf("expected 200 home, got %d %q", rr.Code, rr.Body.String())
    }

    // Subpath should strip prefix
    req2 := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
    rr2 := httptest.NewRecorder()
    r.ServeHTTP(rr2, req2)
    if rr2.Code != http.StatusOK || rr2.Body.String() != "dash" {
        t.Fatalf("expected 200 dash, got %d %q", rr2.Code, rr2.Body.String())
    }
}

func TestMiddlewareOrder(t *testing.T) {
    r := New()
    order := []string{}
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            order = append(order, "a")
            next.ServeHTTP(w, req)
            order = append(order, "d")
        })
    })
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            order = append(order, "b")
            next.ServeHTTP(w, req)
            order = append(order, "c")
        })
    })
    r.GetFunc("/x", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })

    req := httptest.NewRequest(http.MethodGet, "/x", nil)
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)

    got := strings.Join(order, "")
    want := "abcd" // first Use is outermost
    if got != want {
        t.Fatalf("unexpected middleware order: got %q want %q", got, want)
    }
}

func TestMountWithTrailingSlash(t *testing.T) {
    r := New()
    sub := New()
    sub.GetFunc("/", func(w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, "home")
    })
    sub.GetFunc("/dashboard", func(w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, "dash")
    })

    r.Mount("/admin/", sub)

    // Exact mount path should reach sub at "/"
    req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK || rr.Body.String() != "home" {
        t.Fatalf("expected 200 home, got %d %q", rr.Code, rr.Body.String())
    }

    // Subpath should strip prefix
    req2 := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
    rr2 := httptest.NewRecorder()
    r.ServeHTTP(rr2, req2)
    if rr2.Code != http.StatusOK || rr2.Body.String() != "dash" {
        t.Fatalf("expected 200 dash, got %d %q", rr2.Code, rr2.Body.String())
    }
}
