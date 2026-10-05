// Package httpclient builds the HTTP client used to hit target APIs.
package httpclient

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"superiorapi/internal/security"
)

// ErrTooManyRedirects is returned when a target redirects more than allowed.
var ErrTooManyRedirects = errors.New("too many redirects")

// Options configures a benchmark client.
type Options struct {
	Policy       security.Policy
	Concurrency  int
	Timeout      time.Duration // whole request, including reading the body
	MaxRedirects int
}

// New returns a client with a fresh transport, so every test starts with cold
// connections and results are not skewed by connections left over from other
// tests. Callers should call CloseIdleConnections when the test finishes.
func New(o Options) *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   o.Policy.DialControl, // SSRF enforcement on every connect
	}
	transport := &http.Transport{
		Proxy:                 nil, // never route through environment proxies
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          o.Concurrency * 2,
		MaxIdleConnsPerHost:   o.Concurrency,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: o.Timeout,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   o.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > o.MaxRedirects {
				return ErrTooManyRedirects
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect to %s scheme", security.ErrBlocked, req.URL.Scheme)
			}
			return nil
		},
	}
}
