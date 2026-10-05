// Package security guards outbound requests against SSRF.
//
// Two layers are applied:
//  1. ValidateURL / CheckHost reject obviously bad targets at submission time
//     so the user gets a clear error message.
//  2. DialControl runs on every socket connect, after DNS resolution. It is
//     the real enforcement point: it covers DNS rebinding (the name resolving
//     to a public IP during validation and a private one later) and redirects
//     to internal hosts, because every connection is checked by actual IP.
package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
)

// ErrBlocked is returned (wrapped) whenever a destination is not allowed.
var ErrBlocked = errors.New("destination not allowed")

// ErrDNS is returned when the target host cannot be resolved.
var ErrDNS = errors.New("host could not be resolved")

// Policy describes which destinations outbound requests may reach.
type Policy struct {
	// AllowPrivate disables IP and hostname blocking. Only for local development.
	AllowPrivate bool
	// AllowedPorts restricts destination ports. Empty means any port.
	AllowedPorts map[int]bool
}

var blockedPrefixes = mustPrefixes(
	// IPv4
	"0.0.0.0/8",          // "this network"
	"10.0.0.0/8",         // private
	"100.64.0.0/10",      // carrier-grade NAT
	"127.0.0.0/8",        // loopback
	"169.254.0.0/16",     // link-local, includes cloud metadata 169.254.169.254
	"172.16.0.0/12",      // private
	"192.0.0.0/24",       // IETF protocol assignments
	"192.0.2.0/24",       // documentation
	"192.88.99.0/24",     // 6to4 relay anycast
	"192.168.0.0/16",     // private
	"198.18.0.0/15",      // benchmarking
	"198.51.100.0/24",    // documentation
	"203.0.113.0/24",     // documentation
	"224.0.0.0/4",        // multicast
	"240.0.0.0/4",        // reserved
	"255.255.255.255/32", // broadcast
	// IPv6
	"::/128",         // unspecified
	"::1/128",        // loopback
	"64:ff9b::/96",   // NAT64, can embed private IPv4
	"64:ff9b:1::/48", // local-use NAT64
	"100::/64",       // discard
	"2001::/32",      // Teredo, embeds IPv4
	"2001:db8::/32",  // documentation
	"2002::/16",      // 6to4, embeds IPv4
	"fc00::/7",       // unique local, includes fd00:ec2::254 (AWS metadata)
	"fe80::/10",      // link-local
	"ff00::/8",       // multicast
)

var blockedHostnames = []string{"localhost", "metadata.google.internal", "metadata"}

var blockedHostSuffixes = []string{".localhost", ".local", ".internal", ".localdomain", ".home.arpa"}

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}

// IPAllowed reports whether ip is a public unicast address we may connect to.
func (p Policy) IPAllowed(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	if p.AllowPrivate {
		return true
	}
	ip = ip.Unmap() // ::ffff:127.0.0.1 must be treated as 127.0.0.1
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func (p Policy) portAllowed(port int) bool {
	return len(p.AllowedPorts) == 0 || p.AllowedPorts[port]
}

// ValidateURL checks scheme, credentials, hostname and port of a target URL.
// It does not resolve DNS; use CheckHost for that.
func (p Policy) ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http and https URLs are supported")
	}
	if u.User != nil {
		return nil, errors.New("credentials in the URL are not supported; use an Authorization header")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return nil, errors.New("URL must include a host")
	}

	port, err := urlPort(u)
	if err != nil {
		return nil, err
	}
	if !p.portAllowed(port) {
		return nil, fmt.Errorf("%w: port %d is not allowed", ErrBlocked, port)
	}

	if p.AllowPrivate {
		return u, nil
	}
	for _, h := range blockedHostnames {
		if host == h {
			return nil, fmt.Errorf("%w: %s is an internal host", ErrBlocked, host)
		}
	}
	for _, s := range blockedHostSuffixes {
		if strings.HasSuffix(host, s) {
			return nil, fmt.Errorf("%w: %s is an internal host", ErrBlocked, host)
		}
	}
	if ip, err := netip.ParseAddr(host); err == nil && !p.IPAllowed(ip) {
		return nil, fmt.Errorf("%w: %s is a private or reserved address", ErrBlocked, host)
	}
	return u, nil
}

func urlPort(u *url.URL) (int, error) {
	if s := u.Port(); s != "" {
		port, err := strconv.Atoi(s)
		if err != nil || port < 1 || port > 65535 {
			return 0, fmt.Errorf("invalid port %q", s)
		}
		return port, nil
	}
	if u.Scheme == "https" {
		return 443, nil
	}
	return 80, nil
}

// CheckHost resolves host and fails if it does not resolve or if any of its
// addresses is not allowed. This gives early, friendly errors; DialControl
// still enforces the policy at connect time.
func (p Policy) CheckHost(ctx context.Context, host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !p.IPAllowed(ip) {
			return fmt.Errorf("%w: %s is a private or reserved address", ErrBlocked, host)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("%w: %s", ErrDNS, host)
	}
	for _, a := range addrs {
		if !p.IPAllowed(a) {
			return fmt.Errorf("%w: %s resolves to a private or reserved address (%s)", ErrBlocked, host, a.Unmap())
		}
	}
	return nil
}

// DialControl is a net.Dialer Control hook. It runs right before each socket
// connects, with the already-resolved IP address.
func (p Policy) DialControl(network, address string, _ syscall.RawConn) error {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBlocked, err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: unparseable address %q", ErrBlocked, host)
	}
	if !p.IPAllowed(ip) {
		return fmt.Errorf("%w: %s is a private or reserved address", ErrBlocked, ip.Unmap())
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || !p.portAllowed(port) {
		return fmt.Errorf("%w: port %s is not allowed", ErrBlocked, portStr)
	}
	return nil
}
