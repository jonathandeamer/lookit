# Do not use `/W` verbose Finger queries by default

Date: 2026-06-06
Status: accepted

## Context

RFC 1288 defines `/W` as an optional query token. A server should interpret it
as a request for more verbose user information, or ignore it if it does not
support the token.

Live checks against prominent active Finger servers in June 2026 showed that
this is not how much of the modern Finger ecosystem behaves. Classic
`fingerd`-style hosts may honor `/W`, but many modern/custom servers treat the
token as part of the requested username or command. That can return an error,
help text, a fallback profile, a different service result, or a 404-like
response instead of a more detailed version of the same target.

The clearest useful cases found were `tilde.institute` and `tilde.pink`, where
`finger -l @host` expands a compact logged-in-user table into detailed records
for all logged-in users. Manual follow-up confirmed that this is host-list
behavior only: named user lookups such as `finger -s conorh@tilde.institute`
and `finger -l conorh@tilde.institute`, or `finger -s benchhard@tilde.pink` and
`finger -l benchhard@tilde.pink`, return the same information.

## Decision

`lookit` should not send `/W` automatically for either host queries or named
user/service queries.

The default exploration path should stay:

1. Query `@host` with the normal short query.
2. Prefer a compact, parseable directory or logged-in-user list when the server
   provides one.
3. Let the user choose a target from that list.
4. Fetch the chosen user or service directly.

If verbose querying is ever exposed, it should be an explicit advanced/manual
action, not part of normal browsing or automatic retry behavior.

## Rationale

Verbose host listings are more useful in a one-shot Finger CLI than in an
exploration-and-browsing TUI. A CLI user may want "tell me everything now" in a
single terminal dump. `lookit` is optimized for browsing: a compact host list is
more scannable, easier to parse into selectable entries, and avoids pulling
large `.plan` content before the user has chosen a person or service.

Sending `/W` by default would make the common path worse on both sides:

- On classic `fingerd` hosts, it can replace a concise directory with a long
  page of expanded records.
- On modern custom servers, it often changes the target semantics or fails.

That makes `/W` a poor default even though it is part of the RFC query grammar.
Support in the wild is narrow, and the known useful behavior does not match
`lookit`'s default interaction model.

## Consequences

- Host-list parsing should continue to be based on normal `@host` responses.
- User/service drilling should continue to send the selected target directly,
  without adding `/W`.
- Future verbose support, if added, should be opt-in and clearly framed as a
  manual retry/raw-detail action.
- Documentation should describe `/W` as a deliberately deferred/niche protocol
  option, not an accidental omission.
