// Package api is the Sermo REST client of sermo-cli. It wraps the generated client in package gen.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// defaultTimeout is the request timeout when Options.HTTP is nil.
const defaultTimeout = 30 * time.Second

// Options configure a Client.
type Options struct {
	// Server is the base URL of the Sermo server, for example https://fcuba.net.
	Server string
	// Token is the bearer token. An empty Token sends requests as a guest.
	Token string
	// UserAgent is the value of the User-Agent header.
	UserAgent string
	// HTTP is the HTTP client to use. When it is nil, the Client makes its own.
	HTTP *http.Client
	// Sleep waits before a retry after a 429. When it is nil, the Client uses a timer.
	Sleep func(context.Context, time.Duration) error
}

// Client calls the Sermo API. It keeps a cache of site metadata for its life. It is safe for concurrent use.
type Client struct {
	gen    *gen.ClientWithResponses
	http   *http.Client
	server string
	cache  cache
}

// New returns a Client for o.Server.
func New(o Options) (*Client, error) {
	server := strings.TrimRight(o.Server, "/")
	if server == "" {
		return nil, errors.New("api: server URL is empty")
	}
	hc := &http.Client{Timeout: defaultTimeout}
	if o.HTTP != nil {
		copied := *o.HTTP
		hc = &copied
	}
	base := hc.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	sleep := o.Sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	hc.Transport = &transport{base: base, token: o.Token, userAgent: o.UserAgent, sleep: sleep}
	g, err := gen.NewClientWithResponses(server, gen.WithHTTPClient(hc))
	if err != nil {
		return nil, err
	}
	return &Client{gen: g, http: hc, server: server}, nil
}

// doJSON sends a request for an operation that package gen does not have.
// It sends in as the JSON body when in is not nil, and decodes the response into out.
func (c *Client) doJSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.server+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := check(resp, raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
