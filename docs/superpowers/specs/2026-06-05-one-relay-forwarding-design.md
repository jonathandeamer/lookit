# One-relay RFC 1288 forwarding design

Date: 2026-06-05

## Summary

Add safe, typed-only support for one RFC 1288 forwarding hop.

The motivating address is:

```text
jonathan@tilde.team@thebackupbox.net
```

Per RFC 1288, this is not an email-like address. It is a forwarded Finger
query. `lookit` should connect to the rightmost host, `thebackupbox.net`, and
send the remainder, `jonathan@tilde.team`, as the one-line Finger query.

This design deliberately supports exactly one relay. It does not turn `lookit`
into a general recursive forwarding client, and it does not open forwarded
targets harvested from server responses.

## RFC basis

RFC 1288 section 2.3 defines `{H}` recursively:

```text
{Q2} ::= [{W}{S}][{U}]{H}{C}
{H}  ::= @hostname | @hostname{H}
```

Section 2.4 defines forwarding by removing the rightmost `@hostname` token from
the query, connecting to that rightmost host, and sending the remainder as the
next query.

The important product consequence is:

- `alice@host@relay` means dial `relay:79`, send `alice@host`.
- `@host@relay` means dial `relay:79`, send `@host`.
- `alice@host` remains an ordinary direct query: dial `host:79`, send `alice`.

## Goals

- Support user-typed one-relay forwarding addresses:
  - `user@host@relay`
  - `@host@relay`
  - `user@host@relay:port`
  - `@host@relay:port`
- Make the dial endpoint and wire query explicit in the `finger.Target` model.
- Preserve existing direct target forms and display behavior.
- Keep server-supplied targets pinned to port 79 and reject forwarding there.
- Provide targeted error messages for unsupported or deferred forms.
- Avoid DNS, network lookups, or reachability checks during parsing.

## Non-goals

- Do not support more than one relay in this feature.
- Do not support forwarded ports on the inner host.
- Do not support forwarding in `finger://` URLs yet.
- Do not support server-supplied forwarded targets yet.
- Do not add `/W` support.
- Do not add recursive client-side forwarding. `lookit` dials exactly one host;
  any further forwarding is the relay RUIP's responsibility.

## Accepted and rejected forms

Accepted:

- `alice@example.com` -> dial `example.com:79`, query `alice`
- `@example.com` -> dial `example.com:79`, query empty string
- `alice@example.com:7979` -> dial `example.com:7979`, query `alice`
- `alice@tilde.team@thebackupbox.net` -> dial `thebackupbox.net:79`,
  query `alice@tilde.team`
- `@tilde.team@thebackupbox.net` -> dial `thebackupbox.net:79`,
  query `@tilde.team`
- `alice@tilde.team@thebackupbox.net:7979` -> dial
  `thebackupbox.net:7979`, query `alice@tilde.team`

Rejected:

- `alice@h1@h2@relay`
- `@h1@h2@relay`
- `alice@tilde.team:7979@thebackupbox.net`
- `@tilde.team:7979@thebackupbox.net`
- `finger://thebackupbox.net/alice@tilde.team`
- server-supplied `alice@tilde.team@thebackupbox.net`
- any target containing ASCII C0 controls or DEL in either the wire query or
  the dial host token

## Target model

Make the wire query explicit.

```go
type Target struct {
    User     string // deprecated compatibility alias for Query
    Query    string // exact Finger query line without trailing CRLF
    HostPort string // canonical dial endpoint, always host:port
    Raw      string // normalized user-visible address
}
```

Direct examples:

```go
ParseTarget("alice@example.com")
// Query: "alice"
// HostPort: "example.com:79"
// Raw: "alice@example.com"

ParseTarget("@example.com")
// Query: ""
// HostPort: "example.com:79"
// Raw: "@example.com"
```

Forwarded examples:

```go
ParseTarget("alice@tilde.team@thebackupbox.net")
// Query: "alice@tilde.team"
// HostPort: "thebackupbox.net:79"
// Raw: "alice@tilde.team@thebackupbox.net"

ParseTarget("@tilde.team@thebackupbox.net")
// Query: "@tilde.team"
// HostPort: "thebackupbox.net:79"
// Raw: "@tilde.team@thebackupbox.net"
```

For compatibility during migration, code keeps `User` as a deprecated alias for
`Query`, but `finger.Query` must write `Target.QueryLine()`, not `Target.User`.

## Parser shape

Keep `ParseTarget` as the single typed-target chokepoint.

1. Reject empty input.
2. Detect and reject deferred URL forwarding before generic normalization:
   `finger://relay/user@host`.
3. Normalize existing accepted non-forwarding scheme/path forms as today.
4. Reject ASCII controls and DEL in the full normalized target.
5. Count `@` tokens in the normalized target.
6. For one `@`, parse as a direct target:
   - `user@host[:port]`
   - `@host[:port]`
   - dial `host[:port]`
   - query `user` or empty string
7. For two `@` tokens, parse as one-relay forwarding:
   - split at the rightmost `@`
   - the left side is the wire query
   - the right side is the relay host token
   - validate the left side as either `user@host` or `@host`
   - reject any port in the inner host
   - parse the relay with the existing host/port rules
8. For more than two `@` tokens, return the multi-relay unsupported error.

Use the same bracketed IPv6 and port validation for the relay host as direct
targets. The inner forwarded host should accept the same host syntax except
ports are not allowed.

## Server-supplied targets

`ParseTargetPinned` is for targets lifted from server responses, such as
`finger://` links or `finger user@host` text.

It must continue to reject forwarding. A remote server response should not be
able to cause `lookit` to ask one arbitrary host to forward to another arbitrary
host. This keeps the existing security boundary simple:

- typed input may request one relay;
- server text may request only one direct host, pinned to port 79.

When a selected list item contains a server-supplied forwarded target, the TUI
should show a status flash rather than silently doing nothing.

## Error messages

Use targeted errors for deferred or unsupported forms. Avoid describing valid
protocol shapes as simply invalid.

- More than one relay:
  `forwarding through multiple relays is not supported yet`

- Inner forwarded port:
  `forwarded host ports are not supported; put a port only on the relay`

- URL forwarding:
  `forwarding in finger:// URLs is not supported yet; use user@host@relay`

- Server-supplied forwarded target:
  `forwarded targets from server responses are not opened`

- Generic malformed one-relay forwarding:
  `forwarded targets must be user@host@relay or @host@relay`

Existing host/port errors remain:

- Empty, zero, non-numeric, or out-of-range port:
  `invalid port`
- Unbracketed IPv6:
  `IPv6 literals must be bracketed, e.g. [::1]`
- Control characters:
  `target contains control characters`

## Security invariants

- `finger.Query` writes exactly `Target.QueryLine() + "\r\n"`.
- `finger.Query` rejects controls in `Target.Query` even if a caller constructs
  a `Target` directly.
- `ParseTarget` rejects controls before constructing `Target`.
- `ParseTarget` does not validate by DNS lookup or by opening a connection.
- Only the outer relay may have an explicit port.
- Server-supplied targets are pinned to port 79 and may not use forwarding.
- Response bytes remain sanitized at ingress by `finger.Query`.
- Copying an address copies `Raw`, not a reconstructed query/dial pair.

## UI behavior

Typed forwarding should behave like any other submitted target:

- The input accepts `user@host@relay` and `@host@relay`.
- History stores and refills `Raw`.
- The loading bar shows `Raw`.
- The reader header can initially show `Raw` unchanged.

A later polish pass may render forwarded addresses as `user@host via relay`,
but that is not required for the first implementation. The priority is correct
wire behavior and honest errors.

For server-supplied forwarded targets, pressing Enter on the list item should
not fetch. The status bar should flash:

```text
forwarded targets from server responses are not opened
```

## Testing

Parser tests:

- Direct target behavior remains unchanged.
- `alice@tilde.team@thebackupbox.net` parses to query
  `alice@tilde.team`, host `thebackupbox.net:79`.
- `@tilde.team@thebackupbox.net` parses to query `@tilde.team`, host
  `thebackupbox.net:79`.
- `alice@tilde.team@thebackupbox.net:7979` preserves the relay port.
- More than one relay returns the targeted multi-relay error.
- Inner ports return the targeted inner-port error.
- URL forwarding returns the targeted URL-forwarding error.
- Malformed forwarding returns the generic forwarding-shape error.
- Controls anywhere in the normalized target return the control-character
  error.

Wire tests:

- A local fake server receives `alice@tilde.team\r\n` when querying
  `alice@tilde.team@<fake server address>`.
- A local fake server receives `@tilde.team\r\n` when querying
  `@tilde.team@<fake server address>`.
- Direct queries still send only the direct user token.

TUI tests:

- Submitting a typed one-relay target starts a fetch with the relay `HostPort`
  and forwarded `Query`.
- Drilling a server-supplied forwarded target does not fetch and flashes the
  server-supplied forwarding message.
- Copying a server-supplied forwarded target copies nothing and flashes the
  same server-supplied forwarding refusal.

## Deferred follow-up

Forwarding in `finger://` URLs is intentionally deferred.

The likely future shape is:

```text
finger://relay/user@host
```

meaning dial `relay:79` and send `user@host`. That needs a URL-aware parser
instead of the current lightweight normalization pass, because `@` has URL
userinfo meaning in some contexts and Finger's forwarded query syntax gives it
a protocol-specific meaning in the path. Until that work is designed, the
parser should reject URL forwarding with:

```text
forwarding in finger:// URLs is not supported yet; use user@host@relay
```
