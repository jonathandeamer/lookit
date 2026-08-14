# Connection failure copy

Issue: #75

## Context

A failed connection renders the string Go's dialer produced, unedited:

```
dial 127.0.0.1:1: dial tcp 127.0.0.1:1: connect: connection refused
```

`finger.queryWith` wraps the dialer error with `fmt.Errorf("dial %s: %w", …)`,
and `net` has already written its own `dial tcp <addr>: connect:` prefix, so the
address appears twice and the word "dial" appears twice. What remains after the
scaffolding — the address and the reason — is the only part the user asked
about. The read path has the same shape: `read response timed out after 30s:
read tcp 127.0.0.1:53012->127.0.0.1:2479: i/o timeout`.

This is the least considered copy in the app, on the screen where the user is
already having a bad time. It reaches the terminal through two paths:
`render.RenderWithWidth`, which draws the error line in the reader viewport,
and `requestFailure.priorityStatus`, which puts `err.Error()` in the status
bar's `detail` slot — so shorter, denser text helps twice.

## Decision

Classify the failures lookit understands and give each one a sentence in
lookit's voice. Leave every unclassified failure carrying its original text.

The classification rule is the honesty guarantee: **lookit only replaces error
text it has positively identified.** An unrecognised failure is summarised no
further than "which address, which operation", with the underlying text kept
verbatim, so a summarised failure is never a lost failure. Classified kinds
lose nothing either — the raw string's information (address plus reason) is
exactly what the replacement states.

### Where it lives

A new `finger/errors.go` defines `*QueryError`, returned by `queryWith` in place
of today's `fmt.Errorf` wrappers:

```go
type QueryError struct {
    Op      string       // "dial" or "read"
    Addr    string       // the target's host:port
    Host    string       // host without the port, for name failures
    Kind    FailureKind  // refused, timeout, no-such-host, unreachable, unknown
    Timeout time.Duration // the limit that expired; zero otherwise
    Err     error        // the underlying error, preserved
}
```

`Error()` renders the sentence. `Unwrap()` returns `Err`, so `errors.Is` and
`errors.As` keep working against `syscall.ECONNREFUSED`, `*net.DNSError`,
`net.Error` and friends.

`finger/` is the right home: it is the layer that holds the `net` error values
to classify, it already emits display-bound text (`read response timed out
after 30s`), and putting classification anywhere else would mean re-parsing
error strings. `render/` and `tui/` keep printing `err.Error()` and need no
change.

Classification is a pure function of an error value — `classifyDial(err)` and
`classifyRead(err)` — so it is unit-testable against constructed `net` errors
without opening a socket.

### The copy

With `addr` = `127.0.0.1:1` and `host` = `nosuchhost.example`:

| Kind | Detected by | Message |
| --- | --- | --- |
| refused | `errors.Is(err, syscall.ECONNREFUSED)` | `connection refused by 127.0.0.1:1` |
| no such host | `errors.As(&*net.DNSError)`, `IsNotFound` | `no such host: nosuchhost.example` |
| dns failure | `*net.DNSError`, not `IsNotFound` | `couldn't look up nosuchhost.example: server misbehaving` |
| network unreachable | `errors.Is(err, syscall.ENETUNREACH)` | `network unreachable: 127.0.0.1:1` |
| host unreachable | `errors.Is(err, syscall.EHOSTUNREACH)` | `host unreachable: 127.0.0.1:1` |
| dial timeout | `net.Error.Timeout()`, or `context.DeadlineExceeded` | `no answer from 127.0.0.1:1 after 10s` |
| read timeout | `net.Error.Timeout()` on the read | `127.0.0.1:1 stopped responding after 30s` |
| unknown dial | anything else | `couldn't reach 127.0.0.1:1: <underlying>` |
| unknown read | anything else | `couldn't read from 127.0.0.1:1: <underlying>` |

The dns-failure line uses the `*net.DNSError`'s own `Err` field — the reason
alone — rather than the whole error, whose `Error()` already renders as
`lookup <name>: <reason>` and would state the host twice. Repeating the address
is the specific flaw this change exists to remove, so it must not reappear in
the one message that quotes an underlying error's text.

Each is a lowercase fragment with no trailing period, matching the app's other
in-body and status copy, and each is short enough to survive the 45-column
status bar's `detail` slot with the address still visible.

The three failure modes the issue calls out — refused, timed out, name not
resolved — are the three that get distinct words. They imply different next
actions (nothing is answering that port; it is answering too slowly; the name
does not resolve at all), and the shared `r retry` remains correct for all
three, so no new action is introduced.

### Cancellation is not a failure

`queryWith` already returns `ctx.Err()` unwrapped when the caller cancels, and
the TUI drops a canceled request's result rather than rendering it. That path
is unchanged; a `context.Canceled` dial error is returned as `ctx.Err()`, never
dressed up as a connection failure.

## What does not change

- `render/`, which still renders `queryErr.Error()` and wraps only that line at
  the reader's width.
- The `Meta` values, truncation semantics, the 1 MiB cap, the reset-after-body
  rule, and every success path.
- The security invariants: sanitize at ingress, `hasControl` at egress, and
  port-79 pinning for server-supplied targets. Error text is lookit-authored or
  `net`-authored; it never carries response bytes.
- `set deadline`, `write query`, and `query contains control characters`, which
  are already lookit-shaped and are not connection outcomes a user meets in
  normal use.
- The status bar's decision about *which* fields to show on a failed request,
  which is issue #76 and a separate change.

## Testing

- Table-driven tests over `classifyDial`/`classifyRead` with constructed
  `*net.OpError`, `*net.DNSError`, `syscall.Errno` and timeout errors, asserting
  both the kind and the exact rendered sentence.
- An unclassified error keeps its underlying text inside the message.
- `errors.Is(queryErr, syscall.ECONNREFUSED)` and `errors.As` into
  `*net.DNSError` still succeed through `QueryError`.
- The existing local-listener tests still pass unchanged, including the
  read-timeout test, whose expected text becomes the new sentence.
- One end-to-end test dials a closed loopback port and asserts the rendered
  message, using no network beyond loopback (matching existing tests).
