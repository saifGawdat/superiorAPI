// Package config loads server settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"superiorapi/internal/testrun"
)

// Config holds all server settings.
type Config struct {
	Port              string
	AllowedOrigins    []string // CORS origins; "*" allows any
	ProbeRegion       string   // shown to users: where requests originate
	TrustProxyHeaders bool     // use X-Forwarded-For for the client IP

	AllowPrivateTargets bool         // local development only: disables SSRF IP blocking
	AllowedPorts        map[int]bool // empty = any port

	MaxRequests      int
	MaxConcurrency   int
	DefaultTimeout   time.Duration
	MaxTimeout       time.Duration
	MaxBodyBytes     int
	MaxResponseBytes int64
	MaxRedirects     int
	MaxHeaders       int

	Limits testrun.Limits
}

// Load reads the configuration from the environment, applying defaults.
func Load() (Config, error) {
	e := &env{}
	c := Config{
		Port:                e.str("PORT", "8080"),
		AllowedOrigins:      e.list("ALLOWED_ORIGINS", "http://localhost:3000"),
		ProbeRegion:         e.str("PROBE_REGION", "local"),
		TrustProxyHeaders:   e.bool("TRUST_PROXY_HEADERS", false),
		AllowPrivateTargets: e.bool("ALLOW_PRIVATE_TARGETS", false),
		AllowedPorts:        e.ports("ALLOWED_PORTS", "80,443,8080,8443"),
		MaxRequests:         e.int("MAX_REQUESTS", 500),
		MaxConcurrency:      e.int("MAX_CONCURRENCY", 50),
		DefaultTimeout:      e.ms("DEFAULT_TIMEOUT_MS", 10000),
		MaxTimeout:          e.ms("MAX_TIMEOUT_MS", 30000),
		MaxBodyBytes:        e.int("MAX_BODY_BYTES", 64<<10),
		MaxResponseBytes:    int64(e.int("MAX_RESPONSE_BYTES", 5<<20)),
		MaxRedirects:        e.int("MAX_REDIRECTS", 5),
		MaxHeaders:          e.int("MAX_HEADERS", 50),
		Limits: testrun.Limits{
			MaxActiveGlobal:    e.int("MAX_ACTIVE_TESTS", 20),
			MaxActivePerClient: e.int("MAX_ACTIVE_TESTS_PER_CLIENT", 1),
			MaxActivePerTarget: e.int("MAX_ACTIVE_TESTS_PER_TARGET", 2),
			MaxTestsPerHour:    e.int("MAX_TESTS_PER_HOUR", 30),
			MaxTestDuration:    e.ms("MAX_TEST_DURATION_MS", 120000),
			ResultTTL:          e.ms("RESULT_TTL_MS", 30*60*1000),
		},
	}
	if e.err != nil {
		return Config{}, e.err
	}
	if c.DefaultTimeout > c.MaxTimeout {
		return Config{}, fmt.Errorf("DEFAULT_TIMEOUT_MS must not exceed MAX_TIMEOUT_MS")
	}
	return c, nil
}

// env reads variables and remembers the first parse error.
type env struct{ err error }

func (e *env) str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func (e *env) list(key, def string) []string {
	var out []string
	for _, s := range strings.Split(e.str(key, def), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (e *env) int(key string, def int) int {
	s := e.str(key, strconv.Itoa(def))
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		e.fail(fmt.Errorf("%s must be a positive integer, got %q", key, s))
		return def
	}
	return v
}

func (e *env) ms(key string, def int) time.Duration {
	return time.Duration(e.int(key, def)) * time.Millisecond
}

func (e *env) bool(key string, def bool) bool {
	s := e.str(key, strconv.FormatBool(def))
	v, err := strconv.ParseBool(s)
	if err != nil {
		e.fail(fmt.Errorf("%s must be true or false, got %q", key, s))
		return def
	}
	return v
}

func (e *env) ports(key, def string) map[int]bool {
	raw := e.str(key, def)
	if raw == "*" {
		return nil
	}
	out := map[int]bool{}
	for _, s := range e.list(key, def) {
		p, err := strconv.Atoi(s)
		if err != nil || p < 1 || p > 65535 {
			e.fail(fmt.Errorf("%s contains invalid port %q", key, s))
			continue
		}
		out[p] = true
	}
	return out
}

func (e *env) fail(err error) {
	if e.err == nil {
		e.err = err
	}
}
