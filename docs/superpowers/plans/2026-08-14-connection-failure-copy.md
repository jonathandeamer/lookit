# Connection Failure Copy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Go's raw dialer text (`dial 127.0.0.1:1: dial tcp 127.0.0.1:1: connect: connection refused`) with a classified sentence in lookit's voice, while keeping every unrecognised failure's original text.

**Architecture:** A new `finger/errors.go` adds `*QueryError` — op, address, classified kind, the timeout that expired, and the wrapped underlying error. A pure `classify(error) FailureKind` maps `net`/`syscall` values to kinds, so classification is unit-testable without a socket. `queryWith` returns `*QueryError` in place of its `fmt.Errorf` wrappers. `render/` and `tui/` are unchanged: they already print `err.Error()`.

**Tech Stack:** Go standard library only (`errors`, `net`, `syscall`, `context`, `time`).

**Spec:** `docs/superpowers/specs/2026-08-14-connection-failure-copy-design.md`

## Global Constraints

- `finger/`'s security invariants are untouched: `sanitize` at ingress, `hasControl` at egress, the 1 MiB body cap, CRLF→LF normalisation, the reset-after-body rule, and port-79 pinning. Error text is lookit-authored or `net`-authored and never carries response bytes.
- Every message is a lowercase fragment with no trailing period.
- **Only positively classified failures get lookit's words.** `FailureUnknown` must include the underlying error's text verbatim.
- `*QueryError` must implement `Unwrap() error` so `errors.Is`/`errors.As` keep reaching `syscall.ECONNREFUSED`, `*net.DNSError`, and `net.Error`.
- Tests never hit the network. Loopback listeners and constructed error values only.
- No new dependencies. Conventional Commits; no `Co-Authored-By` and no AI attribution trailers.
- Do not push or open a PR.

---

### Task 1: The classified error type

**Files:**
- Create: `finger/errors.go`
- Test: `finger/errors_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `type FailureKind int` with constants `FailureUnknown`, `FailureRefused`, `FailureNoSuchHost`, `FailureDNS`, `FailureNetworkUnreachable`, `FailureHostUnreachable`, `FailureTimeout`; `type QueryError struct{ Op, Addr, Host string; Kind FailureKind; Timeout time.Duration; Err error }` with `Error() string` and `Unwrap() error`; constants `opDial = "dial"` and `opRead = "read"`; and `func newQueryError(op, addr string, timeout time.Duration, err error) *QueryError`.

- [ ] **Step 1: Write the failing tests**

Create `finger/errors_test.go`:

```go
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
```

Note on the `dns failure` expectation: `(*net.DNSError).Error()` renders as
`lookup <name>: <err>`. Run the test and use the exact string Go produces; if it
differs, fix the expectation, not the implementation.

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./finger -run 'TestQueryError' -count=1 -v`

Expected: FAIL — `newQueryError`, `QueryError`, `FailureKind`, `opDial`, and `opRead` are undefined.

- [ ] **Step 3: Implement the type**

Create `finger/errors.go`:

```go
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
```

- [ ] **Step 4: Run the tests and verify GREEN**

Run: `go test ./finger -run 'TestQueryError' -count=1 -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add finger/errors.go finger/errors_test.go
git commit -m "feat(finger): classify connection failures into a typed error"
```

---

### Task 2: Return the classified error from queryWith

**Files:**
- Modify: `finger/client.go` (the dial branch and both read-error branches in `queryWith`)
- Test: `finger/client_test.go`

**Interfaces:**
- Consumes: `newQueryError`, `opDial`, `opRead` from Task 1.
- Produces: `queryWith` returning `*QueryError` for dial failures and read failures.

- [ ] **Step 1: Write the failing tests**

Add to `finger/client_test.go`. The closed-port dial uses loopback only — no
network:

```go
func TestQueryDialRefusedMessage(t *testing.T) {
	// A listener opened and closed immediately gives a port nothing is on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	target, err := ParseTarget("nobody@" + addr)
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	_, _, queryErr := Query(context.Background(), target)
	if queryErr == nil {
		t.Fatal("expected a dial failure against a closed port")
	}
	want := "connection refused by " + addr
	if queryErr.Error() != want {
		t.Fatalf("Error() = %q, want %q", queryErr.Error(), want)
	}
	var qe *QueryError
	if !errors.As(queryErr, &qe) || qe.Op != opDial {
		t.Fatalf("want a *QueryError with Op=%q, got %#v", opDial, queryErr)
	}
}
```

Then find the existing read-timeout test (search `client_test.go` for `read
response timed out`) and update its expected text to the new sentence:
`<addr> stopped responding after <timeout>`, built from the test's own listener
address and read timeout.

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./finger -count=1 -v`

Expected: FAIL — the dial message is still `dial <addr>: dial tcp …` and the read-timeout expectation does not match.

- [ ] **Step 3: Replace the wrappers in `queryWith`**

In `finger/client.go`:

1. Dial branch — return the classified error, and return a bare `ctx.Err()`
   when the caller cancelled, matching what the read path already does (a
   cancellation is not a connection failure):

```go
	conn, err := d.DialContext(dialCtx, "tcp", t.HostPort)
	if err != nil {
		meta.Elapsed = time.Since(start)
		if ctxErr := ctx.Err(); ctxErr != nil {
			// The caller cancelled (esc, or the session ending). That is not a
			// connection failure and must not be dressed up as one.
			return nil, meta, ctxErr
		}
		return nil, meta, newQueryError(opDial, t.HostPort, opts.connectTimeout, err)
	}
```

2. Read-timeout branch:

```go
		if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
			meta.Truncated = true
			return body, meta, newQueryError(opRead, t.HostPort, opts.readTimeout, readErr)
		}
```

3. Final read branch:

```go
		return body, meta, newQueryError(opRead, t.HostPort, 0, readErr)
```

Leave `set deadline`, `write query`, and `query contains control characters`
exactly as they are: they are already lookit-shaped and are not connection
outcomes a user meets in normal use.

- [ ] **Step 4: Run the finger tests and verify GREEN**

Run: `go test ./finger -count=1 -race`

Expected: PASS.

- [ ] **Step 5: Check nothing downstream matched on the old text**

Run: `rg -n "dial tcp|dial %s|read response timed out" --glob '!docs/**' .`

Expected: no matches in `finger/`, `render/`, `tui/`, or `main.go`. Test
fixtures that merely use a made-up error string (for example
`tui/reader_test.go`'s `dialErrText`, `render/wrap_test.go`'s `errDial`) are
inputs, not assertions about `finger`; update them to the new copy only if
their surrounding test reads as a claim about what lookit shows. If you update
`tui/reader_test.go`'s `dialErrText`, keep the test's intent — it checks that a
long reason is not clipped at 60 columns — so replace it with a message long
enough to still exercise wrapping, such as
`"couldn't reach a-very-long-hostname.example.org:79: some unclassified reason"`.

- [ ] **Step 6: Run the full gate and commit**

```bash
make check
git add finger/client.go finger/client_test.go
git commit -m "fix(finger): report connection failures in lookit's voice"
```

---

## Verification

- [ ] `make check` passes.
- [ ] `./lookit` is not required; the change is covered by tests.
- [ ] A closed-port dial renders `connection refused by 127.0.0.1:1` and nothing else.
- [ ] An unclassified failure still shows its original text.
