# httplib

A collection of lightweight, focused helpers for building REST APIs in Go.

## Features

### Router
Lightweight wrapper over net/http's ServeMux with route grouping and nested mounts. Provides chi-like syntax with improved performance.

### Middlewares
Production-ready middleware components:
- `RequestID` - Generate unique request identifiers
- `RealIP` - Extract real client IP from headers
- `Logger` - Structured request logging
- `Recoverer` - Panic recovery with error handling
- `Timeout` - Request timeout management
- `NoCache` - Cache control headers
- `CORS` - Cross-origin resource sharing

### Context Helpers
Utilities for accessing middleware values:
- `GetReqID` - Retrieve request ID from context
- `GetRealIP` - Retrieve real IP from context

### JSON Renderer
Standardized success and error response envelopes with consistent formatting.

### HTTP Client
Multi-endpoint HTTP client with retry logic and client-side load balancing.

## Installation

Install the router package:
```bash
go get github.com/shkmv/httplib/router
```

Install the HTTP client package:
```bash
go get github.com/shkmv/httplib/client
```

## Router Usage

### Basic Setup

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/shkmv/httplib/router"
    "github.com/shkmv/httplib/router/ctxutil"
    "github.com/shkmv/httplib/router/middleware"
)

func main() {
    r := router.New()

    // Add essential middlewares
    r.Use(
        middleware.RealIP(),
        middleware.RequestID(),
        middleware.Logger(nil),
        middleware.Recoverer(nil),
        middleware.NoCache(),
        middleware.Timeout(5*time.Second, "request timeout"),
        middleware.CORS(),
    )

    // Basic route
    r.GetFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, "pong")
    })

    log.Println("Server listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}
```

### Route Groups

```go
r.Route("/api", func(api *router.Router) {
    api.GetFunc("/users", usersHandler)
    api.PostFunc("/users", createUserHandler)
    api.Route("/v1", func(v1 *router.Router) {
        v1.GetFunc("/status", statusHandler)
    })
})
```

### Nested Routers

```go
admin := router.New()
admin.GetFunc("/dashboard", dashboardHandler)
admin.GetFunc("/users", adminUsersHandler)

// Mount admin router at /admin
r.Mount("/admin", admin)
```

### JSON Responses

```go
func usersHandler(w http.ResponseWriter, r *http.Request) {
    users := []User{{ID: 1, Name: "John"}}
    router.RenderOK(w, r, users)
    // Response: {"data": [{"id": 1, "name": "John"}]}
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
    router.BadRequest(w, r, "invalid_input", "Missing required field", map[string]any{
        "field": "email",
    })
    // Response: {"error": "invalid_input", "message": "Missing required field", "request_id": "...", "details": {"field": "email"}}
}
```

### Accessing Middleware Values

```go
func handler(w http.ResponseWriter, r *http.Request) {
    reqID := ctxutil.GetReqID(r.Context())
    realIP := ctxutil.GetRealIP(r.Context())
    
    log.Printf("Request %s from IP %s", reqID, realIP)
}
```

## HTTP Client Usage

### Basic Client

```go
import "github.com/shkmv/httplib/client"

endpoints := []client.Endpoint{
    {BaseURL: "https://api.example.com", DC: "us"},
    {BaseURL: "https://eu-api.example.com", DC: "eu"},
}

c := client.New(endpoints, client.WithPreferredDC("eu"))

// GET request
var response MyResponse
_, err := c.GetJSON(ctx, "/v1/users", &response)
if err != nil {
    log.Fatal(err)
}

// POST request
request := MyRequest{Name: "John"}
_, err = c.PostJSON(ctx, "/v1/users", request, &response)
if err != nil {
    log.Fatal(err)
}
```

### Client Configuration

```go
c := client.New(endpoints,
    client.WithPreferredDC("eu"),        // Prefer EU datacenter
    client.WithRetries(3),               // Retry failed requests 3 times
    client.WithTimeout(30*time.Second),  // 30 second timeout
)
```

## Examples

A complete example server is available at `example/router/main.go`. Run it with:

```bash
go run example/router/main.go
```

Test the endpoints:
```bash
curl http://localhost:8080/ping
curl http://localhost:8080/api/users
curl http://localhost:8080/api/admin/dashboard
```

## Requirements

- Go 1.21 or later
