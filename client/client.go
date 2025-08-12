package client

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "math/rand"
    "net"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "time"
)

// Endpoint represents one API instance, optionally labeled with a data center.
type Endpoint struct {
    BaseURL string
    DC      string
}

// RetryPolicy controls retry behavior.
type RetryPolicy struct {
    MaxAttempts               int
    RetryOnStatuses           map[int]bool
    RetryOnConnectionErrors   bool
    RetryOnMethods            map[string]bool
    InitialBackoff            time.Duration
    MaxBackoff                time.Duration
    BackoffJitterFraction     float64 // 0.5 => +/-50%
}

// DefaultRetryPolicy returns a conservative default retry policy.
func DefaultRetryPolicy() RetryPolicy {
    return RetryPolicy{
        MaxAttempts:             3,
        RetryOnStatuses:         map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true},
        RetryOnConnectionErrors: true,
        RetryOnMethods: map[string]bool{
            http.MethodGet:     true,
            http.MethodHead:    true,
            http.MethodOptions: true,
            http.MethodDelete:  true,
        },
        InitialBackoff:        100 * time.Millisecond,
        MaxBackoff:            2 * time.Second,
        BackoffJitterFraction: 0.5,
    }
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.hc = hc } }

// WithRetryPolicy sets the retry policy.
func WithRetryPolicy(rp RetryPolicy) Option { return func(c *Client) { c.retry = rp } }

// WithPreferredDC sets a preferred data center label to try first.
func WithPreferredDC(dc string) Option { return func(c *Client) { c.preferredDC = dc } }

// WithHeader adds a default header applied to every request (unless already set).
func WithHeader(k, v string) Option {
    return func(c *Client) {
        if c.headers == nil { c.headers = map[string]string{} }
        c.headers[k] = v
    }
}

// New creates a new Client.
func New(endpoints []Endpoint, opts ...Option) *Client {
    c := &Client{
        endpoints:   make([]Endpoint, len(endpoints)),
        retry:       DefaultRetryPolicy(),
        baseTimeout: 10 * time.Second,
    }
    copy(c.endpoints, endpoints)
    c.bal = newBalancer(c.endpoints)
    c.hc = &http.Client{Timeout: c.baseTimeout, Transport: defaultTransport()}
    c.headers = map[string]string{
        "User-Agent": "httplib-client/1.0",
        "Accept":     "application/json",
    }
    for _, opt := range opts { opt(c) }
    return c
}

// Client is a convenient HTTP client with retry and client-side balancing.
type Client struct {
    hc          *http.Client
    endpoints   []Endpoint
    bal         *balancer
    preferredDC string
    retry       RetryPolicy
    headers     map[string]string
    baseTimeout time.Duration
    mu          sync.Mutex
}

// Do sends the HTTP request, applying base URL from a balanced endpoint, default headers,
// and retry policy. If req.URL is absolute, it is used as-is and no endpoint is selected.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
    if ctx != nil {
        req = req.WithContext(ctx)
    }
    attempts := 0
    var lastErr error

    for {
        attempts++
        // Prepare request for this attempt: apply endpoint if needed and clone body.
        attemptReq, cleanup, err := c.prepareAttempt(req)
        if err != nil { return nil, err }

        // Default headers (do not override if already present)
        for k, v := range c.headers {
            if attemptReq.Header.Get(k) == "" { attemptReq.Header.Set(k, v) }
        }

        // Request-ID: if caller set one in headers, keep it.

        resp, err := c.hc.Do(attemptReq)
        if err == nil && !c.shouldRetry(attemptReq, resp, nil, attempts) {
            if cleanup != nil { cleanup() }
            return resp, nil
        }

        // Decide retry and update balancer health.
        if err != nil { lastErr = err; c.bal.markFailure(attemptReq.URL.Host) } else { c.bal.markFailure(attemptReq.URL.Host); lastErr = fmt.Errorf("status %d", resp.StatusCode) }
        if resp != nil { resp.Body.Close() }
        if cleanup != nil { cleanup() }

        if attempts >= max(1, c.retry.MaxAttempts) || !c.shouldRetry(attemptReq, resp, err, attempts) {
            if err != nil { return nil, err }
            return nil, lastErr
        }

        // Backoff with jitter.
        backoff := backoffWithJitter(c.retry.InitialBackoff, c.retry.MaxBackoff, c.retry.BackoffJitterFraction, attempts-1)
        select {
        case <-time.After(backoff):
        case <-attemptReq.Context().Done():
            return nil, attemptReq.Context().Err()
        }

        // On next attempt, choose next endpoint.
        c.bal.nextHost(c.preferredDC)
    }
}

// prepareAttempt clones the request and applies a base endpoint if req.URL is relative.
// It also rewinds the body for retries by buffering small bodies in-memory.
func (c *Client) prepareAttempt(req *http.Request) (*http.Request, func(), error) {
    // Clone request shallowly.
    r2 := req.Clone(req.Context())

    // Ensure body can be re-read across attempts by buffering if necessary.
    var cleanup func()
    if req.Body != nil {
        // If GetBody is set, use it; otherwise buffer into memory.
        if req.GetBody != nil {
            b, err := req.GetBody()
            if err != nil { return nil, nil, err }
            r2.Body = b
        } else {
            data, err := io.ReadAll(req.Body)
            if err != nil { return nil, nil, err }
            _ = req.Body.Close()
            r2.Body = io.NopCloser(bytes.NewReader(data))
            // reset original req.Body for potential future prepareAttempt calls
            req.Body = io.NopCloser(bytes.NewReader(data))
            cleanup = func() {}
        }
    }

    // If URL is absolute, keep as-is.
    if r2.URL != nil && r2.URL.IsAbs() {
        return r2, cleanup, nil
    }

    // Choose endpoint and resolve URL
    base := c.bal.currentBaseURL(c.preferredDC)
    if base == "" {
        return nil, cleanup, errors.New("no endpoints configured")
    }
    bu, err := url.Parse(base)
    if err != nil { return nil, cleanup, err }
    ref := &url.URL{Path: r2.URL.Path, RawPath: r2.URL.RawPath, RawQuery: r2.URL.RawQuery}
    r2.URL = bu.ResolveReference(ref)
    return r2, cleanup, nil
}

// GetJSON issues a GET to a relative path and unmarshals JSON into out.
func (c *Client) GetJSON(ctx context.Context, path string, out interface{}) (*http.Response, error) {
    req, _ := http.NewRequest(http.MethodGet, path, nil)
    resp, err := c.Do(ctx, req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return resp, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }
    dec := json.NewDecoder(resp.Body)
    return resp, dec.Decode(out)
}

// PostJSON issues a POST with a JSON body and unmarshals JSON into out.
func (c *Client) PostJSON(ctx context.Context, path string, in, out interface{}) (*http.Response, error) {
    var body io.ReadCloser
    if in != nil {
        buf := &bytes.Buffer{}
        if err := json.NewEncoder(buf).Encode(in); err != nil { return nil, err }
        body = io.NopCloser(bytes.NewReader(buf.Bytes()))
    }
    req, _ := http.NewRequest(http.MethodPost, path, body)
    if in != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    resp, err := c.Do(ctx, req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return resp, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }
    if out == nil { io.Copy(io.Discard, resp.Body); return resp, nil }
    dec := json.NewDecoder(resp.Body)
    return resp, dec.Decode(out)
}

func (c *Client) shouldRetry(req *http.Request, resp *http.Response, err error, attempts int) bool {
    if attempts >= max(1, c.retry.MaxAttempts) { return false }
    // Respect context cancellation
    if err != nil {
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return false
        }
        // Network errors
        var netErr net.Error
        if c.retry.RetryOnConnectionErrors && (errors.As(err, &netErr) || isConnRefused(err) || isNoSuchHost(err)) {
            return c.retryOnMethod(req.Method)
        }
        // Other errors: don't retry
        return false
    }

    if resp != nil {
        if c.retry.RetryOnStatuses[resp.StatusCode] {
            return c.retryOnMethod(req.Method)
        }
    }
    return false
}

func (c *Client) retryOnMethod(m string) bool { return c.retry.RetryOnMethods[strings.ToUpper(m)] }

// defaultTransport returns a tuned http.Transport.
func defaultTransport() http.RoundTripper {
    return &http.Transport{
        Proxy: http.ProxyFromEnvironment,
        DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
        ForceAttemptHTTP2:     true,
        MaxIdleConns:          100,
        IdleConnTimeout:       90 * time.Second,
        TLSHandshakeTimeout:   5 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
    }
}

// backoffWithJitter calculates exponential backoff with jitter.
func backoffWithJitter(initial, max time.Duration, jitterFrac float64, attempt int) time.Duration {
    if attempt < 0 { attempt = 0 }
    d := initial * (1 << attempt)
    if d > max { d = max }
    if jitterFrac > 0 {
        // +/- jitterFrac
        jitter := (rand.Float64()*2 - 1) * jitterFrac
        d = time.Duration(float64(d) * (1 + jitter))
        if d < 0 { d = 0 }
    }
    return d
}

func isConnRefused(err error) bool { return strings.Contains(err.Error(), "connection refused") }
func isNoSuchHost(err error) bool { return strings.Contains(err.Error(), "no such host") }

// Balancer with health tracking
type balancer struct {
    eps          []Endpoint
    rrAll        int
    rrPreferred  int
    mu           sync.Mutex
    failures     map[string]int       // host -> consecutive failures
    unhealthyTil map[string]time.Time // host -> time until considered unhealthy
}

func newBalancer(eps []Endpoint) *balancer {
    return &balancer{eps: eps, failures: map[string]int{}, unhealthyTil: map[string]time.Time{}}
}

// currentBaseURL returns baseURL of next host based on RR and preferred DC, skipping unhealthy.
func (b *balancer) currentBaseURL(preferredDC string) string {
    b.mu.Lock()
    defer b.mu.Unlock()
    // Try preferred DC first
    if preferredDC != "" {
        indices := b.indicesWithDC(preferredDC)
        if len(indices) > 0 {
            for i := 0; i < len(indices); i++ {
                idx := indices[b.rrPreferred%len(indices)]
                b.rrPreferred++
                if b.isHealthyHostIdx(idx) { return b.eps[idx].BaseURL }
            }
        }
    }
    // Fallback to all
    for i := 0; i < len(b.eps); i++ {
        idx := b.rrAll % max(1, len(b.eps))
        b.rrAll++
        if b.isHealthyHostIdx(idx) { return b.eps[idx].BaseURL }
    }
    // As a last resort, return first base even if unhealthy
    if len(b.eps) > 0 { return b.eps[b.rrAll%len(b.eps)].BaseURL }
    return ""
}

// nextHost advances RR counters to encourage moving to next on next attempt.
func (b *balancer) nextHost(preferredDC string) {
    b.mu.Lock(); defer b.mu.Unlock()
    if preferredDC != "" && len(b.indicesWithDC(preferredDC)) > 0 { b.rrPreferred++ } else { b.rrAll++ }
}

func (b *balancer) indicesWithDC(dc string) []int {
    out := make([]int, 0, len(b.eps))
    for i, e := range b.eps { if e.DC == dc { out = append(out, i) } }
    return out
}

func (b *balancer) isHealthyHostIdx(i int) bool {
    if i < 0 || i >= len(b.eps) { return false }
    host := hostOf(b.eps[i].BaseURL)
    until, ok := b.unhealthyTil[host]
    if !ok { return true }
    if time.Now().After(until) { delete(b.unhealthyTil, host); b.failures[host] = 0; return true }
    return false
}

func (b *balancer) markFailure(hostport string) {
    b.mu.Lock(); defer b.mu.Unlock()
    host := hostport
    if strings.Contains(host, "/") {
        host = hostOf(host)
    }
    b.failures[host] = b.failures[host] + 1
    // Exponential backoff unhealthy period with cap
    base := 500 * time.Millisecond
    n := b.failures[host]
    d := base * time.Duration(1<<min(5, n))
    if d > 10*time.Second { d = 10 * time.Second }
    b.unhealthyTil[host] = time.Now().Add(d)
}

func hostOf(base string) string {
    u, err := url.Parse(base)
    if err != nil { return base }
    if u.Host != "" { return u.Host }
    return base
}

func max(a, b int) int { if a > b { return a } ; return b }
func min(a, b int) int { if a < b { return a } ; return b }
