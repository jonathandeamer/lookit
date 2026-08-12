# Pre-release safety and security review

Date: 2026-08-12
Status: record

## Context

A safety and security review of the whole repo, scoped to legitimate
release-blocking issues only — real exploit paths against the local user or
their terminal, data corruption, or violations of the security invariants
documented in `AGENTS.md`. The threat model: a TUI client whose attacker is a
malicious finger server or a malicious response/bookmarks file. Style issues,
robustness nits, and attacks requiring the user to type the payload themselves
were explicitly out of scope.

## Verdict

No release-blocking issues found. `govulncheck ./...` reports no
vulnerabilities in dependencies.

## What was verified

### `finger/` — the untrusted-input chokepoint holds

- `sanitize` defangs every terminal-relevant byte class: C0/DEL → caret
  notation, C1 and invalid UTF-8 → `\xXX`, Cf/Zl/Zp → `\u{...}`. CRLF
  normalization runs *before* sanitize and the 1 MiB cap *after*, so no escape
  sequence can be assembled post-filter or split dangerously by the cap.
- The egress `hasControl` guard (rejecting C0/DEL in the outbound query) fires
  twice: in `ParseTarget` and again immediately before the wire write. The
  second check matters because `tui/links.go` builds some `Target` structs
  manually for same-relay forwards, bypassing the parser.
- An absolute read deadline bounds slow-drip servers; `io.LimitedReader`
  hard-caps the body regardless of chunking; only sanitized bytes leave
  `queryWith` on every path, including error paths.
- `ParseTargetPinned` handles ports, IPv6, `finger://` URLs, and forwarding
  forms correctly; a malicious server can steer a drill at most to `host:79` —
  no SSRF to arbitrary ports.

### `render/` — no path around sanitize

- Every production render path receives only the sanitized body; the exported
  `render.Render` has no production caller.
- Field highlighting styles only constant prefixes; theme styles are
  colour-only (no width/truncation that could slice attacker text
  mid-sequence); error and header strings carry no raw server bytes.
- OSC 8 hyperlink wrapping uses scheme-whitelisted tokens taken from sanitized
  body text, so server text cannot forge or break out of the escape sequence.

### `tui/` — sinks and file I/O

- Bookmarks ingress validates UTF-8, rejects C1/Cf/Zl/Zp, and requires
  `ParseTarget` survival. Writes are atomic (temp file + rename) at `0600`,
  directories `0700`, with deliberate final-symlink handling
  (`bookmarkWritePath`).
- All server-supplied drill targets go through `ParseTargetPinned`; the
  same-relay forwarding exception pins port 79 via `net.JoinHostPort`.
- The `v` view-source view shows the sanitized body, not wire bytes.
- Clipboard copies (OSC 52 via bubbletea, base64-encoded) carry only sanitized
  link tokens or control-free target strings. The list-view copy path re-pins
  server targets to port 79 before copying, so a pasted-back address can't be
  steered at another service.
- The only file writes anywhere in `tui/` are the bookmarks file. No `os/exec`
  anywhere in the package.

### main.go, CI, release tooling, repo hygiene

- CLI arg handling quotes offending args with `%q`; the version string is
  derived from git tags or validated module versions — no untrusted input
  reaches it.
- All workflows pin actions to commit SHAs; no `pull_request_target`; PR jobs
  run `contents: read` with no secrets; `HOMEBREW_TAP_GITHUB_TOKEN` is scoped
  to a single step; releases are tag-triggered drafts with build-provenance
  attestation.
- The `commit-msg` hook has no eval/injection path.
- No secrets in the working tree or git history; `dist/` and the binary are
  gitignored.

## Non-blocking observations

- `hasControl` does not reject UTF-8-encoded C1/Cf in *user-typed* targets.
  There is no remote path to this (server-derived targets are extracted from
  sanitized bodies; bookmarks are validated), so it is self-inflicted only.
  Already tracked as issue #49.
- Release binaries are not cosign-signed; provenance attestation plus
  draft-release review is proportionate for a 0.x single-maintainer tool.

## Method note

The review delegated one deep-dive per area (`finger/`, `render/`, `tui/`,
build/CI/hygiene) with instructions to verify the documented invariants
against the code rather than take them on faith, plus a `govulncheck` run.
Findings were required to name file:line, attacker, payload, and impact;
speculative findings were excluded.
