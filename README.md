httplib
================

Lightweight router wrapper over Go's net/http with route grouping, nested mounts, and essential middlewares.

Install
- `go get github.com/shkmv/httplib/router`

Quick Start
- Create router and middlewares:
  - `r := router.New()`
  - `r.Use(router.RealIP(), router.RequestID(), router.Logger(nil), router.Recoverer(nil), router.NoCache())`
  - `r.Use(router.Timeout(5*time.Second, "request timeout"))`
- Routes and groups:
  - `r.GetFunc("/ping", func(w, r) { fmt.Fprint(w, "pong") })`
  - `r.Route("/api", func(api *router.Router) { api.GetFunc("/users", usersHandler) })`
- Mount sub-routers:
  - `admin := router.New(); admin.GetFunc("/dashboard", dashboard)`
  - `r.Mount("/admin", admin)`
- Access values:
  - `router.GetReqID(r.Context())`, `router.GetRealIP(r.Context())`

Example
- See `example/router/main.go` for a runnable example server.

