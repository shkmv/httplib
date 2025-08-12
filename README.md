httplib
================

Collection of small, focused helpers for building REST APIs in Go.

What’s inside (so far)
- Router: Lightweight wrapper over net/http’s ServeMux with route grouping and nested mounts (chi-like) in `github.com/shkmv/httplib/router`.
- Middlewares: RequestID, RealIP, Logger, Recoverer, Timeout, and NoCache (in the same `router` package).
- JSON Renderer: Standard API responses with consistent shapes for success and error (in the `router` package).

Install
- `go get github.com/shkmv/httplib/router`

Quick start
- Setup router and middlewares:
  - `r := router.New()`
  - `r.Use(router.RealIP(), router.RequestID(), router.Logger(nil), router.Recoverer(nil), router.NoCache())`
  - `r.Use(router.Timeout(5*time.Second, "request timeout"))`
- Routes and groups:
  - `r.GetFunc("/ping", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "pong") })`
  - `r.Route("/api", func(api *router.Router) { api.GetFunc("/users", usersHandler) })`
- Mount nested routers:
  - `admin := router.New(); admin.GetFunc("/dashboard", dashboard)`
  - `r.Mount("/admin", admin)`
- JSON rendering:
  - Success: `router.RenderOK(w, r, map[string]any{"hello":"world"}) // {"data":{...}}`
  - Error: `router.BadRequest(w, r, "bad_input", "invalid fields", map[string]any{"field":"name"}) // {"error":...,"message":...,"request_id":...}`
- Access middleware values:
  - `router.GetReqID(r.Context())`, `router.GetRealIP(r.Context())`

Examples
- See `example/router/main.go` for a runnable example server.

Notes
- Tested with Go 1.21+.
