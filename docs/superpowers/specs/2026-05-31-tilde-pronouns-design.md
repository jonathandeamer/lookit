# tilde.team Pronouns highlighting & reflow — design

Date: 2026-05-31

## Summary

tilde.team runs a custom finger daemon that emits three per-user field labels:
`Project:` and `Plan:` (the classic `~/.project` / `~/.plan` blocks) plus a
server-specific `Pronouns:` field (from `~/.pronouns`). lookit highlights
`Project:`/`Plan:` but not `Pronouns:`.

This change gives `Pronouns:` the same treatment as `Plan:`/`Project:` —
**but only for the `tilde.team` host**, since `Pronouns:` is that server's
convention, not a finger standard. Two behaviors, both host-gated:

1. **Highlight** the `Pronouns:` label (same `theme.Field` styling as other fields).
2. **Reflow** an inline `Pronouns: he/him` line into a label-on-its-own-line block
   with the value indented two spaces, matching the visual rhythm of
   `Plan:`/`Project:`.

### Empirical basis

Fingered all 165 deduped users currently on tilde.team. `Project:` and `Plan:`
appear for 164 each; `Pronouns:` for 125 (76%). No other daemon-emitted field
labels exist — everything else (`Website:`, `Matrix:`, `pgp:`, URL schemes,
geek-code, prose ending in a colon) is freeform content inside `.project`/`.plan`.
The classic `~/.pubkey` / `~/.pgpkey` / `~/.forward` labels are **not** emitted by
tilde.team's daemon, so they are out of scope here (would be speculative guesses
at other daemons' label strings).

## Scope decisions (settled)

- Add **only** `Pronouns:` (data-validated). No contact/identity set, no
  `.pubkey`/`.pgpkey` labels.
- Matcher stays **strict and literal**: case-sensitive prefix at line start, no
  URL-scheme guards, no mid-line matching. (The host gate already prevents the
  most likely false positives, since only tilde.team profiles are affected.)
- Both behaviors are gated to the **`tilde.team`** host exactly.

## Architecture

`render.Render(t finger.Target, body, meta, queryErr, profile)` already receives
the target, so the host is available with no signature change. Generic field
highlighting (`Login:`/`Name:`/`Plan:`/`Project:`/…) remains global and untouched.

New host-specific logic lives in one small, named file — `render/tildeteam.go` —
mirroring how the TUI already special-cases `ring@thebackupbox.net`:

- `isTildeTeam(t finger.Target) bool` — strips the port from `t.HostPort`
  (via `net.SplitHostPort`, falling back to the raw value if there's no port) and
  compares to `"tilde.team"` with `strings.EqualFold`.
- `extraFieldPrefixes(t finger.Target) []string` — returns `[]string{"Pronouns:"}`
  when `isTildeTeam(t)`, else `nil`.
- `reflowPronouns(body []byte) []byte` — rewrites an inline `Pronouns: <value>`
  line into block form (see below). Caller only invokes it when `isTildeTeam(t)`.

### Changes to `render/fields.go`

`highlightFields` gains a third parameter:

```go
func highlightFields(theme Theme, body []byte, extra []string) string
```

It matches against `fieldPrefixes` plus `extra`. `fieldPrefixes` itself is
**unchanged** (no global `Pronouns:` entry). Existing callers pass the new
argument; the matching loop is otherwise identical.

### Changes to `render/render.go`

Inside `Render`, in the non-empty-body branch:

```go
if isTildeTeam(t) {
    body = reflowPronouns(body)
}
sb.WriteString(highlightFields(theme, body, extraFieldPrefixes(t)))
```

Data flow: `body → (tilde only) reflowPronouns → highlightFields(theme, body, extra)`.

### `reflowPronouns` behavior

Operates line by line on the body.

- A line equal to `Pronouns: <value>` where `<value>` is non-empty after the
  `Pronouns: ` prefix (one space after the colon) is rewritten to two lines:
  ```
  Pronouns:
    <value>
  ```
  The indent is exactly two spaces, matching the `Plan:` block indent.
- A bare `Pronouns:` line (no value, or only whitespace after the colon) is left
  untouched — there is nothing to move.
- All other lines pass through verbatim. Trailing newline handling matches the
  rest of the body (the function preserves the body's existing line structure).
- The rewrite is purely cosmetic and affects only the rendered output. The TUI's
  raw "view source" (`r`) reads the original body bytes, not `render`, so the
  server's literal formatting stays inspectable — consistent with the project's
  honesty convention.

## Testing

New `render/tildeteam_test.go`:

- `reflowPronouns`: inline `Pronouns: he/him` → `"Pronouns:\n  he/him"`;
  bare `Pronouns:` unchanged; a body with no Pronouns line unchanged; a value with
  internal spaces (`Pronouns: she/her, ask`) reflows with the whole value indented.
- Host gating via `Render`: a body containing `Pronouns: he/him`
  - for target `@tilde.team`: rendered output contains the reflowed block
    `Pronouns:\n  he/him`, and (under a color profile) the `Pronouns:` label is
    wrapped in `theme.Field`.
  - for target `@plan.cat`: rendered output contains the original
    `Pronouns: he/him` line verbatim, unstyled (no reflow, no highlight).
- `isTildeTeam`: true for `tilde.team:79` and `TILDE.TEAM`; false for
  `plan.cat:79` and `nottilde.team`.

## Out of scope

- `~/.pubkey` / `~/.pgpkey` / `~/.forward` labels and any contact/identity labels.
- Case-insensitive or mid-line label matching.
- Any change to generic `fieldPrefixes` or to other hosts' rendering.
