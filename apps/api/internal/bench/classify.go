package bench

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strings"
	"syscall"

	"superiorapi/internal/httpclient"
	"superiorapi/internal/security"
)

// Transport error categories reported in RequestResult.Error.
const (
	ErrTimeout          = "timeout"
	ErrConnRefused      = "connection_refused"
	ErrConnReset        = "connection_reset"
	ErrDNS              = "dns"
	ErrTLS              = "tls"
	ErrBlocked          = "blocked"
	ErrTooManyRedirects = "too_many_redirects"
	ErrOther            = "other"
)

// classifyError maps a client error to a stable category.
func classifyError(err error) string {
	if errors.Is(err, security.ErrBlocked) {
		return ErrBlocked
	}
	if errors.Is(err, httpclient.ErrTooManyRedirects) {
		return ErrTooManyRedirects
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsTimeout {
			return ErrTimeout
		}
		return ErrDNS
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return ErrTimeout
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return ErrConnRefused
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return ErrConnReset
	}
	var certErr *tls.CertificateVerificationError
	var recordErr tls.RecordHeaderError
	var unknownAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	if errors.As(err, &certErr) || errors.As(err, &recordErr) || errors.As(err, &unknownAuth) || errors.As(err, &hostErr) {
		return ErrTLS
	}

	// Windows socket errors don't always unwrap to the syscall constants.
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "actively refused"):
		return ErrConnRefused
	case strings.Contains(msg, "connection reset"), strings.Contains(msg, "forcibly closed"):
		return ErrConnReset
	case strings.Contains(msg, "tls:"), strings.Contains(msg, "x509:"):
		return ErrTLS
	}
	return ErrOther
}
