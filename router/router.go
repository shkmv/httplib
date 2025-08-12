package router

import (
    "net/http"
    "path"
    "strings"
)

// Middleware defines a function to process middleware.
type Middleware func(http.Handler) http.Handler

// Router is a lightweight wrapper around the stdlib http.ServeMux
// that adds route grouping and nested mounting semantics similar to chi.
//
// It shares a single underlying *http.ServeMux across grouped/nested routers
// and implements http.Handler for easy use with http.Server.
type Router struct {
    mux         *http.ServeMux
    base        string
    middlewares []Middleware
}

// New creates a new root Router.
func New() *Router {
    return &Router{mux: http.NewServeMux()}
}

// ServeHTTP satisfies http.Handler by delegating to the underlying mux.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    r.mux.ServeHTTP(w, req)
}

// Use appends middlewares to this router. Middlewares are applied in the
// order they were added, outermost to innermost.
func (r *Router) Use(mws ...Middleware) {
    r.middlewares = append(r.middlewares, mws...)
}

// With returns a shallow copy of the router with additional middlewares appended.
func (r *Router) With(mws ...Middleware) *Router {
    clone := *r
    clone.middlewares = append(append([]Middleware{}, r.middlewares...), mws...)
    return &clone
}

// Route groups routes under a common path prefix.
// Example:
//  r.Route("/api", func(api *router.Router) {
//      api.Get("/ping", handler)
//  })
func (r *Router) Route(prefix string, fn func(*Router)) {
    sub := r.withPrefix(prefix)
    fn(sub)
}

// Group is an alias for Route.
func (r *Router) Group(prefix string, fn func(*Router)) { r.Route(prefix, fn) }

// Mount mounts an http.Handler (another Router or any handler) under prefix.
// Requests to exactly the prefix are rewritten to "/" for the mounted handler.
// Requests to prefix subpaths are served with the prefix stripped.
func (r *Router) Mount(prefix string, h http.Handler) {
    full := r.join(prefix)

    // Exact match redirects path to "/" for the mounted handler.
    r.mux.Handle(full, r.wrap(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        // Clone to avoid mutating original request for other handlers.
        req2 := req.Clone(req.Context())
        req2.URL.Path = "/"
        h.ServeHTTP(w, req2)
    })))

    // Subtree: strip the prefix to make the mounted handler the root.
    subtree := full
    if !strings.HasSuffix(subtree, "/") {
        subtree += "/"
    }
    r.mux.Handle(subtree, r.wrap(http.StripPrefix(strings.TrimRight(full, "/"), h)))
}

// Handle registers a handler for any HTTP method at the full pattern.
// Pattern is joined with any existing group prefix.
func (r *Router) Handle(pattern string, h http.Handler) {
    r.mux.Handle(r.join(pattern), r.wrap(h))
}

// HandleFunc registers a handler func for any HTTP method.
func (r *Router) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Handle(pattern, http.HandlerFunc(h))
}

// Method registers a handler for a specific HTTP method. If the request
// method does not match, it responds with 405 Method Not Allowed.
func (r *Router) Method(method, pattern string, h http.Handler) {
    method = strings.ToUpper(method)
    r.mux.Handle(r.join(pattern), r.wrap(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        if req.Method != method {
            w.Header().Set("Allow", method)
            http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
            return
        }
        h.ServeHTTP(w, req)
    })))
}

// Convenience helpers for common HTTP methods.
func (r *Router) Get(pattern string, h http.Handler)               { r.Method(http.MethodGet, pattern, h) }
func (r *Router) GetFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Get(pattern, http.HandlerFunc(h))
}
func (r *Router) Post(pattern string, h http.Handler)               { r.Method(http.MethodPost, pattern, h) }
func (r *Router) PostFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Post(pattern, http.HandlerFunc(h))
}
func (r *Router) Put(pattern string, h http.Handler)                { r.Method(http.MethodPut, pattern, h) }
func (r *Router) PutFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Put(pattern, http.HandlerFunc(h))
}
func (r *Router) Patch(pattern string, h http.Handler)              { r.Method(http.MethodPatch, pattern, h) }
func (r *Router) PatchFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Patch(pattern, http.HandlerFunc(h))
}
func (r *Router) Delete(pattern string, h http.Handler)             { r.Method(http.MethodDelete, pattern, h) }
func (r *Router) DeleteFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Delete(pattern, http.HandlerFunc(h))
}
func (r *Router) Options(pattern string, h http.Handler)            { r.Method(http.MethodOptions, pattern, h) }
func (r *Router) OptionsFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Options(pattern, http.HandlerFunc(h))
}
func (r *Router) Head(pattern string, h http.Handler)               { r.Method(http.MethodHead, pattern, h) }
func (r *Router) HeadFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
    r.Head(pattern, http.HandlerFunc(h))
}

// internal: create a new router with additional path prefix.
func (r *Router) withPrefix(prefix string) *Router {
    clone := *r
    clone.base = r.join(prefix)
    return &clone
}

// internal: join current base with pattern, producing a clean leading-slash path.
func (r *Router) join(p string) string {
    a := r.base
    if a == "/" {
        a = ""
    }
    if p == "" {
        p = "/"
    }
    // Ensure both have single leading slash semantics when joined.
    joined := path.Join("/"+strings.TrimPrefix(a, "/"), strings.TrimPrefix(p, "/"))
    if joined == "" {
        return "/"
    }
    return joined
}

// internal: apply middleware chain.
func (r *Router) wrap(h http.Handler) http.Handler {
    if len(r.middlewares) == 0 {
        return h
    }
    wrapped := h
    for i := len(r.middlewares) - 1; i >= 0; i-- {
        wrapped = r.middlewares[i](wrapped)
    }
    return wrapped
}

