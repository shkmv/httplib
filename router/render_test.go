package router_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "github.com/shkmv/httplib/router"
    rmid "github.com/shkmv/httplib/router/middleware"
)

func TestRenderData_OK(t *testing.T) {
    r := router.New()
    r.GetFunc("/x", func(w http.ResponseWriter, req *http.Request) {
        router.RenderOK(w, req, map[string]any{"hello": "world"})
    })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
    if rr.Code != http.StatusOK {
        t.Fatalf("status: %d", rr.Code)
    }
    if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
        t.Fatalf("unexpected content type: %q", ct)
    }
    var got struct {
        Data map[string]string `json:"data"`
    }
    if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
        t.Fatalf("json: %v", err)
    }
    if got.Data["hello"] != "world" {
        t.Fatalf("unexpected data: %+v", got)
    }
}

func TestRenderError_WithReqID(t *testing.T) {
    r := router.New()
    r.Use(rmid.RequestID())
    r.GetFunc("/x", func(w http.ResponseWriter, req *http.Request) {
        router.BadRequest(w, req, "bad_input", "invalid fields", map[string]any{"field": "name"})
    })

    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
    if rr.Code != http.StatusBadRequest {
        t.Fatalf("status: %d", rr.Code)
    }
    var got router.ErrorEnvelope
    if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
        t.Fatalf("json: %v", err)
    }
    if got.Error != "bad_input" || got.Message == "" || got.RequestID == "" {
        t.Fatalf("unexpected error envelope: %+v", got)
    }
}
