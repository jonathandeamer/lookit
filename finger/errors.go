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

// FailureKind is the classified reason a query failed. FailureUnknown keeps
// the underlying error's text verbatim.
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

// QueryError is a connection failure in lookit's voice. Err is the original
// net error so errors.Is / errors.As still work.
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
		return fmt.Sprintf("couldn't look up %s: %s", e.Host, dnsReason(e.Err))
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

// dnsReason is just the *net.DNSError reason. Error() already names the host
// as "lookup <name>: <reason>", so quoting that whole string would repeat it.
func dnsReason(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.Err != "" {
		return dnsErr.Err
	}
	return fmt.Sprintf("%v", err)
}

func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
