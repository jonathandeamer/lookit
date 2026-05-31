# Input forms & placeholder rotation — design

Date: 2026-05-31

## Summary

Three independent UX improvements to how `lookit` accepts and hints at targets:

1. **Accept `finger://` scheme and path-style addresses** (`finger://via.sour.is/xuu`,
   `via.sour.is/xuu`) in addition to the existing `user@host` / `@host` forms — in both
   the one-shot CLI and the TUI target box.
2. **Rotate the greyed-out sample input** in the TUI: pick one of five real sample targets
   per launch instead of the static `alice@plan.cat`.
3. Tests for both, with no behavior change to existing forms.

All address-form work lands in one place — `finger.ParseTarget` — because both the CLI
(`main.go`) and the TUI (`tui/app.go`) funnel user input through it. The placeholder work
is TUI-only (`tui/app.go`).

## 1. `finger.ParseTarget` — scheme + path normalization

`ParseTarget` gains a normalization pre-pass that runs **before** the existing `@`-split
logic. The existing logic is unchanged; new forms are rewritten into the forms it already
understands.

### Normalization steps (in order)

1. **Strip scheme.** Case-insensitively strip a leading `finger://`. Also tolerate a bare
   `finger:` prefix (no slashes) by stripping it. After stripping, trim any trailing `/`.
2. **Path form → `@` form.** If the remainder contains **no `@` but does contain `/`**,
   read it as `host[:port]/user` and rewrite to `user@host[:port]`:
   - `via.sour.is/xuu` → `xuu@via.sour.is`
   - `via.sour.is:7979/xuu` → `xuu@via.sour.is:7979` (typed port preserved)
   - `plan.cat/` (empty user) → `@plan.cat` (bare host query)
   - `finger://plan.cat` → (scheme stripped) `plan.cat`, no `/` and no `@` → see step 3.
   - Only the **first** `/` separates host from user. A finger user is a single token, so
     any further `/`-segments are trimmed/ignored.
3. **Bare host after scheme strip.** A remainder with no `@` and no `/` that came from a
   `finger://` input (e.g. `finger://plan.cat`) is treated as a bare `@host` query:
   rewrite to `@<remainder>`. (Without a scheme, a bare `host` with no `@` and no `/`
   still errors, exactly as today — we don't guess that `foo` means `@foo`.)
4. **Everything else unchanged.** `user@host`, `@host`, `:port`, and
   `finger://user@host` (scheme stripped → already contains `@`) fall straight through to
   the existing parser.

### `Raw` normalization

`Target.Raw` is consumed structurally downstream, not just for display — e.g.
`tui/app.go drill()` does `strings.TrimPrefix(host.Raw, "@")` and reconstructs
`login@host`; the status bar splits on `@`. So for any input that went through scheme or
path rewriting, `Raw` is set to the **canonical rewritten string** (`user@host[:port]` or
`@host[:port]`), not the original typed text. Plain `@`-form inputs keep `Raw` exactly as
typed, so existing tests and existing display behavior are untouched.

### Non-goals / safety

- **No new error paths.** Genuinely malformed input (no `@`, no `/`, no scheme) errors
  with the same message as today.
- **`pinFingerPort` is untouched.** That guard pins *server-supplied* targets (lifted from
  a response body) to port 79. These new forms are *user-typed* input, which legitimately
  keeps any explicit port the user wrote — same rule as today's `user@host:port`.

## 2. Rotating placeholder sample (TUI)

At `tui/app.go` (`newApp`, currently `in.Placeholder = "alice@plan.cat"`):

- Introduce an unexported `var sampleTargets = []string{ ... }`:
  - `ring@thebackupbox.net`
  - `@happynetbox.com`
  - `@plan.cat`
  - `@tilde.team`
  - `jonathan@tilde.team`
- A small helper (e.g. `pickSample()`) returns a uniform-random element using `math/rand`
  (Go's auto-seeded global source — no explicit seeding). Random-per-`newApp` rather than a
  persisted cycle: no state to store, and the distribution is even enough for a hint.
- The result is assigned to `in.Placeholder`. It is **placeholder-only** — greyed hint text
  in the empty input, never submitted unless the user types it. The list mixes `@host`
  directory shapes and the `user@host` profile shape (`ring@…`, `jonathan@…`) so the hint
  teaches both input forms.

## 3. Testing

- **`finger/query_test.go`** — extend the table with:
  - `finger://via.sour.is/xuu` → `{User: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"}`
  - `via.sour.is/xuu` (no scheme) → same as above
  - `via.sour.is:7979/xuu` → `{User: "xuu", HostPort: "via.sour.is:7979", Raw: "xuu@via.sour.is:7979"}`
  - `finger://plan.cat` → `{User: "", HostPort: "plan.cat:79", Raw: "@plan.cat"}`
  - `finger://user@host` → `{User: "user", HostPort: "host:79", Raw: "user@host"}`
  - `plan.cat/` (trailing slash, empty user) → `{User: "", HostPort: "plan.cat:79", Raw: "@plan.cat"}`
  - `FINGER://via.sour.is/xuu` (mixed/upper-case scheme) → same as the lower-case case
  - Regression: existing `@`-form rows keep `Raw` equal to the original input.
  - Still-errors: bare `alice`, empty string (unchanged).
- **`tui`** — a test asserting `pickSample()` always returns a member of `sampleTargets`
  (loop a handful of times; membership, not distribution).

## Out of scope

- No changes to `render/`, the userlist parser, or `pinFingerPort`.
- No persisted cycling / usage stats for the placeholder.
- No support for `gopher://`/`http://`-style schemes — finger only.
