package security

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

func TestIPAllowed(t *testing.T) {
	p := Policy{}
	cases := map[string]bool{
		"8.8.8.8":          true,
		"1.1.1.1":          true,
		"2606:4700::1111":  true,
		"127.0.0.1":        false,
		"127.8.9.10":       false,
		"0.0.0.0":          false,
		"10.1.2.3":         false,
		"172.16.0.1":       false,
		"172.31.255.255":   false,
		"192.168.1.1":      false,
		"169.254.169.254":  false,
		"100.64.0.1":       false,
		"224.0.0.1":        false,
		"255.255.255.255":  false,
		"::1":              false,
		"::":               false,
		"::ffff:127.0.0.1": false,
		"::ffff:10.0.0.1":  false,
		"fe80::1":          false,
		"fd00:ec2::254":    false,
		"fc00::1":          false,
		"64:ff9b::a00:1":   false,
		"2002:7f00:1::":    false,
		"ff02::1":          false,
		"::ffff:8.8.8.8":   true,
	}
	for addr, want := range cases {
		if got := p.IPAllowed(netip.MustParseAddr(addr)); got != want {
			t.Errorf("IPAllowed(%s) = %v, want %v", addr, got, want)
		}
	}
	if !(Policy{AllowPrivate: true}).IPAllowed(netip.MustParseAddr("127.0.0.1")) {
		t.Error("AllowPrivate should allow loopback")
	}
}

func TestValidateURL(t *testing.T) {
	p := Policy{AllowedPorts: map[int]bool{80: true, 443: true, 8080: true}}
	cases := []struct {
		url     string
		ok      bool
		blocked bool
	}{
		{"https://api.example.com/users", true, false},
		{"http://example.com:8080/x", true, false},
		{"https://8.8.8.8/", true, false},
		{"ftp://example.com", false, false},
		{"file:///etc/passwd", false, false},
		{"https://user:pass@example.com", false, false},
		{"https://", false, false},
		{"http://localhost:8080", false, true},
		{"http://LOCALHOST.", false, true},
		{"http://foo.localhost", false, true},
		{"http://metadata.google.internal/computeMetadata/v1/", false, true},
		{"http://printer.local", false, true},
		{"http://127.0.0.1", false, true},
		{"http://[::1]/", false, true},
		{"http://169.254.169.254/latest/meta-data/", false, true},
		{"http://10.0.0.5", false, true},
		{"http://example.com:22", false, true},
		{"http://example.com:99999", false, false},
	}
	for _, c := range cases {
		_, err := p.ValidateURL(c.url)
		if c.ok != (err == nil) {
			t.Errorf("ValidateURL(%q) err = %v, want ok=%v", c.url, err, c.ok)
			continue
		}
		if c.blocked != errors.Is(err, ErrBlocked) {
			t.Errorf("ValidateURL(%q) err = %v, want blocked=%v", c.url, err, c.blocked)
		}
	}
}

func TestDialControl(t *testing.T) {
	p := Policy{AllowedPorts: map[int]bool{443: true}}
	if err := p.DialControl("tcp", "8.8.8.8:443", nil); err != nil {
		t.Errorf("public address rejected: %v", err)
	}
	for _, addr := range []string{"127.0.0.1:443", "[::1]:443", "10.0.0.1:443", "169.254.169.254:443", "8.8.8.8:22"} {
		if err := p.DialControl("tcp", addr, nil); !errors.Is(err, ErrBlocked) {
			t.Errorf("DialControl(%s) = %v, want ErrBlocked", addr, err)
		}
	}
}

func TestCheckHostLiteral(t *testing.T) {
	p := Policy{}
	if err := p.CheckHost(context.Background(), "127.0.0.1"); !errors.Is(err, ErrBlocked) {
		t.Errorf("CheckHost(127.0.0.1) = %v, want ErrBlocked", err)
	}
	if err := p.CheckHost(context.Background(), "localhost"); err == nil {
		t.Error("CheckHost(localhost) should fail (resolves to loopback)")
	}
}
