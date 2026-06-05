# Precise target parsing — design

Date: 2026-06-05

## Summary

Tighten `finger.ParseTarget` so target parsing fails early and honestly for
unsupported or malformed inputs, while preserving the current product model:
`lookit` sends one direct finger query to one host and does not implement RFC
1288 forwarding yet.

This combines two related improvements in one branch:

1. Explicitly reject forwarded query forms such as `alice@host1@host2` because
   they are valid RFC 1288 shapes that `lookit` does not support yet.
2. Replace the loose `strings.Contains(hostport, ":")` port-defaulting check
   with precise host/port parsing, including bracketed IPv6 literals.

Prior art: `reiver/go-finger` models RFC 1288 query decomposition, including
multi-hop forwarding and actor/path/address separation. This change is
influenced by that protocol framing, but keeps `lookit`'s current documented
no-forwarding behavior and uses original implementation.

Concretely, the borrowed lesson is conceptual: a target with multiple `@`
tokens is a forwarding query shape, and a target is easier to reason about when
the query actor and address portion are parsed deliberately. No code, tests, or
API shape are copied. The bracketed-IPv6 handling, port-zero rejection, and
error-message choices are `lookit`-specific decisions; `go-finger` does not
provide the IPv6 parser this branch specifies.

Forwarding is not treated as malformed or inherently invalid. It is an RFC 1288
query shape and may become useful later, including for real workflows such as
interacting with `ring@thebackupbox.net`. This branch only makes the current
behavior honest: recognize the forwarding shape, reject it before dialing, and
tell the user that `lookit` does not support it yet.

## Goals

- Keep `finger.ParseTarget` as the single chokepoint for user-typed and
  server-supplied targets.
- Preserve existing accepted forms:
  - `user@host`
  - `@host`
  - `user@host:port`
  - `@host:port`
  - `finger://host/user`
  - `host[:port]/user`
- Add bracketed IPv6 support:
  - `user@[::1]` -> `HostPort: "[::1]:79"`
  - `user@[::1]:7979` -> `HostPort: "[::1]:7979"`
- Reject malformed ports before dialing.
- Reject unbracketed IPv6 with a targeted parse error.
- Turn the existing forwarded-query rough edge into a targeted parse error.

## Parser Shape

`normalizeTarget` keeps its current narrow job: rewrite accepted scheme/path
forms into canonical `user@host` or `@host` strings. It does not validate
hosts or ports.

`ParseTarget` then:

1. Rejects empty input.
2. Runs `normalizeTarget`.
3. Splits once on the first `@`.
4. Rejects missing `@` and empty host as today.
5. Rejects an additional `@` in the host part with an explicit forwarding
   error, e.g. `forwarded finger queries are not supported yet`.
6. Rejects ASCII control characters in the user and pre-expansion host/port
   text as today.
7. Calls `parseHostPort` to produce the canonical dial address stored in
   `Target.HostPort`.
8. Returns `Target{User, HostPort, Raw}` where `Raw` is the normalized target
   string, not the port-expanded dial address.

For the `ParseTarget` path, keeping `Raw` unexpanded preserves display and
history behavior:
`alice@example.com` stays `alice@example.com`, not `alice@example.com:79`;
`alice@[::1]` stays `alice@[::1]`.

This is not a global `Raw` invariant. The server-supplied-target pinning path
already expands `Raw` after rewriting the port (`rawFromTarget` returns
`User + "@" + HostPort`). That pre-existing behavior is intentionally
unchanged. It remains correct for IPv6 because `HostPort` is canonical and
already bracketed, so `rawFromTarget(finger.Target{User: "alice", HostPort:
"[::1]:79"})` produces `alice@[::1]:79`.

## Host/Port Rules

Add an unexported helper:

```go
func parseHostPort(s string) (string, error)
```

Rules:

- `example.com` -> `example.com:79`
- `example.com:7979` -> `example.com:7979`
- `[::1]` -> `[::1]:79`
- `[::1]:7979` -> `[::1]:7979`
- `example.com:` -> error
- `example.com:abc` -> error
- `example.com:99999` -> error
- `example.com:0` -> error
- `::1` -> error: IPv6 literals must use brackets
- `[::1` -> error

Port validation is decimal `1..65535`. Port 0 is valid as a local bind
sentinel, but it is not a meaningful remote finger service port, so typed
targets reject it before dialing. The parser should avoid DNS or network
lookups; it validates syntax only.

Use an explicit syntax split rather than routing on `net.SplitHostPort` error
strings:

- If `s` starts with `[`, find the first `]`. Missing `]` is an error. The host
  is the bracketed literal. The remaining suffix is either empty (default
  `:79`) or `:<port>`. Any other suffix is an error.
- Otherwise, count `:` in `s`.
  - Zero colons: bare host, default `:79`.
  - One colon: `host:port`; host and port must both be non-empty.
  - More than one colon: reject as an unbracketed IPv6 literal.

Validate any explicit port with `strconv.ParseUint(port, 10, 16)`, then reject
`0`. Output should use `net.JoinHostPort` or an equivalent bracket-preserving
path so the dial string is canonical.

`normalizeTarget` needs no IPv6-specific rewrite: the existing first-`/` split
is safe because IPv6 literals do not contain `/`, and bracketed IPv6 remains in
the host portion (`[::1]/alice` -> `alice@[::1]`).

## Error Handling

Errors should be targeted enough to explain the user's typo:

- Forwarding: forwarded finger queries are not supported yet.
- Empty port / non-numeric port / zero / out-of-range port: invalid port.
- Unbracketed IPv6: IPv6 literals must be bracketed, e.g. `[::1]`.

The exact text can be short, but tests should assert at least representative
error cases return errors. Existing higher-level UI already displays
`ParseTarget` errors through the input/status flow.

## Tests

Extend `finger/query_test.go` with table cases:

- Existing accepted forms still pass unchanged.
- `alice@plan.cat@tilde.team` errors.
- `@plan.cat@tilde.team` errors.
- `alice@example.com:` errors.
- `alice@example.com:abc` errors.
- `alice@example.com:99999` errors.
- `alice@example.com:0` errors.
- `alice@::1` errors.
- `alice@[::1` errors.
- `alice@[::1]:` errors.
- `alice@[::1]:abc` errors.
- `alice@[::1]` parses to `User: "alice"`, `HostPort: "[::1]:79"`,
  `Raw: "alice@[::1]"`.
- `alice@[::1]:7979` parses to `HostPort: "[::1]:7979"`.
- `@[::1]` parses to `User: ""`, `HostPort: "[::1]:79"`, `Raw: "@[::1]"`.
- `@[::1]:7979` parses to `User: ""`, `HostPort: "[::1]:7979"`,
  `Raw: "@[::1]:7979"`.
- `finger://[::1]/alice` parses to `Raw: "alice@[::1]"`.
- `[::1]/alice` parses to `Raw: "alice@[::1]"`.
- `[::1]:7979/alice` parses to `HostPort: "[::1]:7979"`,
  `Raw: "alice@[::1]:7979"`.

No live-network tests are needed. Existing `Query` tests continue to cover
writing the parsed `Target.User` to a local fake server.

Add a focused TUI test for the pinning path:

- `pinFingerPort(finger.Target{User: "alice", HostPort: "[::1]:2222",
  Raw: "alice@[::1]:2222"})` returns `HostPort: "[::1]:79"` and
  `Raw: "alice@[::1]:79"`.

This guards the security-adjacent server-target port pinning path without
rewriting it.

## Non-Goals

- Do not implement RFC 1288 forwarding in this branch. The improved error is a
  placeholder for future support, not a permanent product statement.
- Do not add `/W` support.
- Do not accept unbracketed IPv6 heuristically.
- Do not rewrite the TUI drill model, history model, or `pinFingerPort`.
- Do not add a dependency on `go-finger`.
