httplib
================

Collection of small, focused helpers for building REST APIs in Go.

What’s inside (so far)
- Router: Lightweight wrapper over net/http’s ServeMux with route grouping and nested mounts (chi-like) in `github.com/shkmv/httplib/router`.
- Middlewares: `RequestID`, `RealIP`, `Logger`, `Recoverer`, `Timeout`, `NoCache`, `CORS` in `github.com/shkmv/httplib/router/middleware`.
- Context helpers: `GetReqID`, `GetRealIP` in `github.com/shkmv/httplib/router/ctxutil`.
- JSON Renderer: Standard success/error envelopes in `github.com/shkmv/httplib/router`.
- HTTP Client: Retry + client-side load-balancing in `github.com/shkmv/httplib/client`.

Install
- Router: `go get github.com/shkmv/httplib/router`
- Client: `go get github.com/shkmv/httplib/client`

Quick start
- Setup router and middlewares:
  - `r := router.New()`
  - `r.Use(middleware.RealIP(), middleware.RequestID(), middleware.Logger(nil), middleware.Recoverer(nil), middleware.NoCache(), middleware.Timeout(5*time.Second, "request timeout"), middleware.CORS())`
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
  - `ctxutil.GetReqID(r.Context())`, `ctxutil.GetRealIP(r.Context())`

Client quick start
- Multiple endpoints with DC preference and retries:
  - `eps := []client.Endpoint{{BaseURL: "https://eu.api", DC: "eu"}, {BaseURL: "https://us.api", DC: "us"}}`
  - `c := client.New(eps, client.WithPreferredDC("eu"))`
  - `var out Resp; _, _ = c.GetJSON(ctx, "/v1/resource?id=1", &out)`
  - `in := map[string]any{"name":"demo"}; _, _ = c.PostJSON(ctx, "/v1/items", in, &out)`

Examples
- See `example/router/main.go` for a runnable example server.

Notes
- Tested with Go 1.21+.
