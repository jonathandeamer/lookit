package finger

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

// dialErr builds the error shape the dialer really produces: an OpError
// wrapping an os.SyscallError wrapping the errno.
func dialErr(errno syscall.Errno) error {
	return &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &os.SyscallError{Syscall: "connect", Err: errno},
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestQueryErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		op      string
		addr    string
		timeout time.Duration
		err     error
		kind    FailureKind
		want    string
	}{
		{
			name: "refused",
			op:   opDial, addr: "127.0.0.1:1",
			err:  dialErr(syscall.ECONNREFUSED),
			kind: FailureRefused,
			want: "connection refused by 127.0.0.1:1",
		},
		{
			name: "no such host",
			op:   opDial, addr: "nosuchhost.example:79",
			err:  &net.DNSError{Err: "no such host", Name: "nosuchhost.example", IsNotFound: true},
			kind: FailureNoSuchHost,
			want: "no such host: nosuchhost.example",
		},
		{
			name: "dns failure",
			op:   opDial, addr: "nosuchhost.example:79",
			err:  &net.DNSError{Err: "server misbehaving", Name: "nosuchhost.example"},
			kind: FailureDNS,
			want: "couldn't look up nosuchhost.example: server misbehaving",
		},
		{
			name: "dns failure without a reason falls back to the full error text",
			op:   opDial, addr: "nosuchhost.example:79",
			err:  &net.DNSError{Err: "", Name: "nosuchhost.example"},
			kind: FailureDNS,
			want: "couldn't look up nosuchhost.example: lookup nosuchhost.example: ",
		},
		{
			name: "network unreachable",
			op:   opDial, addr: "10.0.0.1:79",
			err:  dialErr(syscall.ENETUNREACH),
			kind: FailureNetworkUnreachable,
			want: "network unreachable: 10.0.0.1:79",
		},
		{
			name: "host unreachable",
			op:   opDial, addr: "10.0.0.1:79",
			err:  dialErr(syscall.EHOSTUNREACH),
			kind: FailureHostUnreachable,
			want: "host unreachable: 10.0.0.1:79",
		},
		{
			name: "dial timeout",
			op:   opDial, addr: "tilde.team:79", timeout: 10 * time.Second,
			err:  fmt.Errorf("dial tcp: %w", context.DeadlineExceeded),
			kind: FailureTimeout,
			want: "no answer from tilde.team:79 after 10s",
		},
		{
			name: "read timeout",
			op:   opRead, addr: "tilde.team:79", timeout: 30 * time.Second,
			err:  timeoutError{},
			kind: FailureTimeout,
			want: "tilde.team:79 stopped responding after 30s",
		},
		{
			name: "unknown dial failure keeps the underlying text",
			op:   opDial, addr: "tilde.team:79",
			err:  errors.New("something nobody classified"),
			kind: FailureUnknown,
			want: "couldn't reach tilde.team:79: something nobody classified",
		},
		{
			name: "unknown read failure keeps the underlying text",
			op:   opRead, addr: "tilde.team:79",
			err:  errors.New("connection reset by peer"),
			kind: FailureUnknown,
			want: "couldn't read from tilde.team:79: connection reset by peer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := newQueryError(tc.op, tc.addr, tc.timeout, tc.err)
			if got.Kind != tc.kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tc.kind)
			}
			if got.Error() != tc.want {
				t.Errorf("Error() = %q, want %q", got.Error(), tc.want)
			}
		})
	}
}

func TestQueryErrorUnwraps(t *testing.T) {
	refused := newQueryError(opDial, "127.0.0.1:1", 0, dialErr(syscall.ECONNREFUSED))
	if !errors.Is(refused, syscall.ECONNREFUSED) {
		t.Error("errors.Is must still reach syscall.ECONNREFUSED through QueryError")
	}

	dnsErr := &net.DNSError{Err: "no such host", Name: "nosuchhost.example", IsNotFound: true}
	wrapped := newQueryError(opDial, "nosuchhost.example:79", 0, dnsErr)
	var target *net.DNSError
	if !errors.As(wrapped, &target) || target.Name != "nosuchhost.example" {
		t.Error("errors.As must still reach *net.DNSError through QueryError")
	}
}
