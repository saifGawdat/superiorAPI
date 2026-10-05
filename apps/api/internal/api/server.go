// Package api exposes the profiler over HTTP: start, inspect, stream and
// cancel tests.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"superiorapi/internal/bench"
	"superiorapi/internal/config"
	"superiorapi/internal/security"
	"superiorapi/internal/testrun"
)

// Server handles API requests.
type Server struct {
	cfg     config.Config
	policy  security.Policy
	manager *testrun.Manager
}

// New returns the HTTP handler for the API.
func New(cfg config.Config, policy security.Policy, manager *testrun.Manager) http.Handler {
	s := &Server{cfg: cfg, policy: policy, manager: manager}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/limits", s.limits)
	mux.HandleFunc("POST /api/tests", s.startTest)
	mux.HandleFunc("GET /api/tests/{id}", s.getTest)
	mux.HandleFunc("GET /api/tests/{id}/events", s.streamTest)
	mux.HandleFunc("POST /api/tests/{id}/cancel", s.cancelTest)
	return s.cors(mux)
}

type apiError struct {
	status  int
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, e *apiError) {
	writeJSON(w, e.status, map[string]*apiError{"error": e})
}

func invalid(field, msg string) *apiError {
	return &apiError{status: http.StatusBadRequest, Code: "invalid_request", Message: msg, Field: field}
}

func (s *Server) limits(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"maxRequests":      s.cfg.MaxRequests,
		"maxConcurrency":   s.cfg.MaxConcurrency,
		"maxTimeoutMs":     s.cfg.MaxTimeout.Milliseconds(),
		"defaultTimeoutMs": s.cfg.DefaultTimeout.Milliseconds(),
		"maxBodyBytes":     s.cfg.MaxBodyBytes,
		"probeRegion":      s.cfg.ProbeRegion,
	})
}

type startRequest struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        *string           `json:"body"`
	Requests    int               `json:"requests"`
	Concurrency int               `json:"concurrency"`
	TimeoutMs   *int              `json:"timeoutMs"`
}

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

func (s *Server) startTest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxBodyBytes)*2+64<<10)
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, invalid("", "The request body must be valid JSON."))
		return
	}
	cfg, host, e := s.buildConfig(r.Context(), req)
	if e != nil {
		writeError(w, e)
		return
	}

	client := s.clientKey(r)
	t, err := s.manager.Start(cfg, client, host)
	var rl *testrun.RateLimitError
	if errors.As(err, &rl) {
		writeError(w, &apiError{status: http.StatusTooManyRequests, Code: "rate_limited", Message: rl.Error()})
		return
	}
	if err != nil {
		writeError(w, &apiError{status: http.StatusInternalServerError, Code: "internal", Message: "Could not start the test."})
		return
	}
	slog.Info("test started", "id", t.ID, "host", host, "method", cfg.Method,
		"requests", cfg.Requests, "concurrency", cfg.Concurrency, "client", client)
	writeJSON(w, http.StatusAccepted, map[string]string{"testId": t.ID})
}

// buildConfig applies the limits from the spec and the SSRF policy.
func (s *Server) buildConfig(ctx context.Context, req startRequest) (bench.Config, string, *apiError) {
	u, err := s.policy.ValidateURL(req.URL)
	if err != nil {
		if errors.Is(err, security.ErrBlocked) {
			return bench.Config{}, "", &apiError{status: http.StatusBadRequest, Code: "target_not_allowed", Message: err.Error(), Field: "url"}
		}
		return bench.Config{}, "", invalid("url", err.Error())
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = "GET"
	}
	if !slices.Contains(methods, method) {
		return bench.Config{}, "", invalid("method", "Method must be one of "+strings.Join(methods, ", ")+".")
	}
	if req.Requests < 1 || req.Requests > s.cfg.MaxRequests {
		return bench.Config{}, "", invalid("requests", fmt.Sprintf("Requests must be between 1 and %d.", s.cfg.MaxRequests))
	}
	if req.Concurrency < 1 || req.Concurrency > s.cfg.MaxConcurrency {
		return bench.Config{}, "", invalid("concurrency", fmt.Sprintf("Concurrency must be between 1 and %d.", s.cfg.MaxConcurrency))
	}
	if req.Concurrency > req.Requests {
		return bench.Config{}, "", invalid("concurrency", "Concurrency cannot be higher than the number of requests.")
	}
	timeout := s.cfg.DefaultTimeout
	if req.TimeoutMs != nil {
		timeout = time.Duration(*req.TimeoutMs) * time.Millisecond
		if timeout < 100*time.Millisecond || timeout > s.cfg.MaxTimeout {
			return bench.Config{}, "", invalid("timeoutMs", fmt.Sprintf("Timeout must be between 100 and %d ms.", s.cfg.MaxTimeout.Milliseconds()))
		}
	}
	if len(req.Headers) > s.cfg.MaxHeaders {
		return bench.Config{}, "", invalid("headers", fmt.Sprintf("At most %d headers are allowed.", s.cfg.MaxHeaders))
	}
	body := req.Body
	if body != nil && *body == "" {
		body = nil
	}
	if body != nil && len(*body) > s.cfg.MaxBodyBytes {
		return bench.Config{}, "", invalid("body", fmt.Sprintf("The body must be at most %d bytes.", s.cfg.MaxBodyBytes))
	}

	host := strings.ToLower(u.Hostname())
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.policy.CheckHost(rctx, host); err != nil {
		if errors.Is(err, security.ErrBlocked) {
			return bench.Config{}, "", &apiError{status: http.StatusBadRequest, Code: "target_not_allowed", Message: err.Error(), Field: "url"}
		}
		return bench.Config{}, "", &apiError{status: http.StatusBadRequest, Code: "dns_failed", Message: "Could not resolve " + host + ".", Field: "url"}
	}

	return bench.Config{
		URL: u.String(), Method: method, Headers: req.Headers, Body: body,
		Requests: req.Requests, Concurrency: req.Concurrency, Timeout: timeout,
	}, host, nil
}

func (s *Server) lookup(w http.ResponseWriter, r *http.Request) (*testrun.Test, bool) {
	t, ok := s.manager.Get(r.PathValue("id"))
	if !ok {
		writeError(w, &apiError{status: http.StatusNotFound, Code: "not_found", Message: "Test not found or expired."})
	}
	return t, ok
}

func (s *Server) getTest(w http.ResponseWriter, r *http.Request) {
	if t, ok := s.lookup(w, r); ok {
		writeJSON(w, http.StatusOK, t.Snapshot())
	}
}

func (s *Server) cancelTest(w http.ResponseWriter, r *http.Request) {
	if t, ok := s.lookup(w, r); ok {
		t.Cancel()
		writeJSON(w, http.StatusAccepted, struct{}{})
	}
}

// clientKey identifies the caller for rate limiting. Behind a trusted proxy
// the right-most X-Forwarded-For entry is the one the proxy itself added.
func (s *Server) clientKey(r *http.Request) string {
	if s.cfg.TrustProxyHeaders {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (slices.Contains(s.cfg.AllowedOrigins, origin) || slices.Contains(s.cfg.AllowedOrigins, "*")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
