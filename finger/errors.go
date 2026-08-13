package finger

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// Operations a QueryError can describe.
const (
	opDial = "dial"
	opRead = "read"
)

// FailureKind is the classified reason a query failed.
//
// Only kinds lookit recognises are given lookit's own words. FailureUnknown
// keeps the underlying error's text verbatim, so a summarised failure is never
// a lost failure — the guarantee that makes it safe to rewrite the rest.
type FailureKind int

const (
	FailureUnknown FailureKind = iota
	FailureRefused
	FailureNoSuchHost
	FailureDNS
	FailureNetworkUnreachable
	FailureHostUnreachable
	FailureTimeout
)

// QueryError is a connection failure in lookit's voice. Go's dialer produces
// text like "dial tcp 127.0.0.1:1: connect: connection refused", which repeats
// the address and exposes its own call structure; the useful content is the
// address and the reason. The original error is kept in Err, so errors.Is and
// errors.As still work and nothing is diagnostically lost.
//
// This lives in finger/ because finger/ is the layer holding the net error
// values to classify; classifying anywhere else would mean parsing error
// strings.
type QueryError struct {
	Op      string        // opDial or opRead
	Addr    string        // the target's host:port
	Host    string        // host without the port, for name failures
	Kind    FailureKind   // what lookit recognised, if anything
	Timeout time.Duration // the limit that expired; zero otherwise
	Err     error         // the underlying error, preserved
}

func newQueryError(op, addr string, timeout time.Duration, err error) *QueryError {
	return &QueryError{
		Op:      op,
		Addr:    addr,
		Host:    hostOnly(addr),
		Kind:    classify(err),
		Timeout: timeout,
		Err:     err,
	}
}

func (e *QueryError) Unwrap() error { return e.Err }

func (e *QueryError) Error() string {
	switch e.Kind {
	case FailureRefused:
		return "connection refused by " + e.Addr
	case FailureNoSuchHost:
		return "no such host: " + e.Host
	case FailureDNS:
		return fmt.Sprintf("couldn't look up %s: %v", e.Host, e.Err)
	case FailureNetworkUnreachable:
		return "network unreachable: " + e.Addr
	case FailureHostUnreachable:
		return "host unreachable: " + e.Addr
	case FailureTimeout:
		if e.Op == opRead {
			return fmt.Sprintf("%s stopped responding after %s", e.Addr, e.Timeout)
		}
		return fmt.Sprintf("no answer from %s after %s", e.Addr, e.Timeout)
	default:
		if e.Op == opRead {
			return fmt.Sprintf("couldn't read from %s: %v", e.Addr, e.Err)
		}
		return fmt.Sprintf("couldn't reach %s: %v", e.Addr, e.Err)
	}
}

// classify is a pure function of an error value so it can be tested against
// constructed net errors without opening a socket.
func classify(err error) FailureKind {
	// DNSError also satisfies net.Error, so it must be examined before the
	// generic timeout check below.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return FailureNoSuchHost
		}
		return FailureDNS
	}
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return FailureRefused
	case errors.Is(err, syscall.ENETUNREACH):
		return FailureNetworkUnreachable
	case errors.Is(err, syscall.EHOSTUNREACH):
		return FailureHostUnreachable
	case errors.Is(err, context.DeadlineExceeded):
		return FailureTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return FailureTimeout
	}
	return FailureUnknown
}

func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
