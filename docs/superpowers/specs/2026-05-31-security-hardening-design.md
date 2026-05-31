# v0.1.0 security & safety hardening design

## Goal

Close the six findings from the v0.1.0 security/safety review so the MVP ships
"feature complete for what it is, without obvious unintended flaws." Each fix
treats finger responses — and server-supplied navigation targets derived from
them — as untrusted input, and tightens the boundary between that input and the
user's terminal, clipboard, and outbound socket.

This is a focused hardening batch: the six findings only, no unrelated
refactoring. Every behavioral change is test-first, using the existing
injected-fake patterns (no real network or TTY in tests). `make check` is the
gate.

The findings split cleanly by package:

| # | Pri | Finding | File |
|---|-----|---------|------|
| 1 | P2 | Escape non-printing Unicode format controls | `finger/sanitize.go` |
| 2 | P3 | Reject control chars in the outbound query | `finger/query.go` |
| 3 | P2 | Pin copied server targets before clipboard | `tui/app.go` |
| 4 | P2 | Drop stale fetch results | `tui/app.go`, `tui/fetch.go` |
| 5 | P2 | Cap parsed user lists | `tui/list.go` |
| 6 | P3 | Restrict CI workflow token permissions | `.github/workflows/*.yml` |

Findings 1 and 2 together complete the "untrusted bytes in, untrusted token
out" boundary: §3.3 ingress filtering already defangs terminal escapes
(see `2026-05-31-rfc1288-control-char-filtering-design.md`); this batch extends
ingress to Unicode reordering/hiding controls (1) and adds the symmetric egress
guard so a hostile drill target can't smuggle a multi-line query (2).

---

## `finger/` package

### 1. Escape non-printing Unicode format controls (P2)

**Threat.** `sanitize`'s slow path defangs only C0/C1/DEL; every other decoded
rune falls through to `b.WriteRune(r)` verbatim. A finger server can therefore
send Unicode *format* controls that reorder or hide text without using any
terminal escape: U+202E RIGHT-TO-LEFT OVERRIDE (visually reverses following
text — classic spoofing of logins/filenames), zero-width space/joiner, BOM /
ZWNBSP, soft hyphen, the bidi isolates, tag characters. These survive today
because they are neither C0/C1 nor DEL, yet they are exactly the "unprintable
data" RFC 1288 §3.3 says to filter.

**Change.** Add one branch to the slow-path `switch` in `sanitize`, before the
`default` arm, that escapes:

- `unicode.Cf` — the Unicode *Format* category. Covers RLO/LRO/LRE/RLE/PDF,
  the directional isolates (U+2066–U+2069), ZWJ/ZWNJ (U+200C/U+200D), LRM/RLM,
  zero-width space-adjacent format chars, word joiner (U+2060), BOM/ZWNBSP
  (U+FEFF), soft hyphen (U+00AD), Arabic format chars, interlinear annotations,
  and the tag block (U+E0001, U+E0020–U+E007F).
- `unicode.Zl` and `unicode.Zp` — LINE SEPARATOR (U+2028) and PARAGRAPH
  SEPARATOR (U+2029), which can break layout.

**Notation.** A new third notation, `\u{XXXX}` (lowercase, variable-width hex),
alongside the existing `^X` (C0/DEL caret) and `\xXX` (C1 / invalid byte). The
`{}` form distinguishes a defanged *rune* from a defanged *byte* and reads
clearly for code points beyond U+FFFF. Add a `writeUnicodeEscape(b, r)` helper
mirroring `writeHex`/`writeCaret`.

**`isClean` is unchanged.** It already returns false for any byte `>= 0x80`,
forcing every multibyte rune onto the slow path where the new classification
lives — so the fast path stays correct and the common pure-ASCII case still
skips allocation. Pure-ASCII bodies contain no Cf/Zl/Zp runes, so the new
branch never fires on the fast path's domain.

**Ordering note.** C1 controls and NEL (U+0085) are caught by the existing
`r >= 0x80 && r <= 0x9f` arm (rendered `\x85` etc.); the new Cf/Zl/Zp arm sits
after it, so nothing is reclassified.

**Tests** (`sanitize_test.go`, extend the table): U+202E RLO, U+200B ZWSP,
U+FEFF BOM, U+200D ZWJ between two emoji stays escaped, U+2028 LINE SEPARATOR,
U+00AD soft hyphen; plus a regression assertion that ordinary accented text and
a plain emoji are still preserved verbatim.

### 2. Reject control chars in the outbound query (P3)

**Threat.** `queryWith` writes `fmt.Fprintf(conn, "%s\r\n", t.User)` with no
validation. A `User` (or host) token containing `\r`/`\n` — reachable from a
server-supplied `finger user@host` drill target parsed out of an untrusted
listing, or from scripted/pasted input — makes lookit emit *multiple* query
lines instead of the single RFC 1288 query it documents (request smuggling
shape). `pinFingerPort` guards the port of such targets but not the token.

**Decision: reject, not strip.** Return a clear error rather than silently
normalizing. Stripping would rewrite `evil\r\nadmin@host` into `eviladmin@host`
— a token that *looks* legitimate but isn't what anything advertised; a hostile
drill target should fail loudly, and a user typo deserves an honest message.

**Change (defense in depth, two points):**

1. **Primary — `ParseTarget` (`query.go`).** After deriving `user` and
   `hostport`, reject either containing a C0 control (`< 0x20`, includes
   `\r`/`\n`) or DEL (`0x7f`), returning
   `errors.New("target contains control characters")`. This is the fail-fast
   path for user-typed and parsed-link input.
2. **Backstop — `queryWith` (`query.go`).** Immediately before the `Fprintf`,
   re-check `t.User` for the same class and return an error if present, so a
   hand-constructed `Target` that never went through `ParseTarget` still cannot
   reach the wire as a multi-line query. Factor the predicate into a small
   unexported `hasControl(string) bool` helper shared by both call sites.

**Scope note.** This validates the ASCII control range only (the
smuggling/injection vector). Full Unicode-token validation is out of scope;
hosts are also constrained downstream by the resolver.

**Tests:** `ParseTarget("a\r\nb@host")` and `ParseTarget("u@ho\x00st")` return
errors; a table case for a clean target still parses; `queryWith` with a
hand-built `Target{User: "a\r\nb", ...}` returns an error and writes nothing
(assert against the local test server / a recording conn).

---

## `tui/` package

### 3. Pin copied server targets before clipboard (P2)

**Threat.** `copyAddress` (`app.go`) stores `sel.target` raw for the `y` key.
Enter routes the same selection through `pinFingerPort` (→ port 79), but `y`
does not: a directory advertising `finger://example.com:22/evil` is safe on
Enter yet copies `evil@example.com:22`, and pasting that back is treated as
user-typed and connects to port 22 — bypassing the server-supplied-target
safety boundary.

**Change.** In `copyAddress`, when the selected item carries a server-supplied
`sel.target`, build a `finger.Target` from it and run `pinFingerPort` before
forming the copied string / flash — mirroring `drill` exactly. The
`login + "@" + host` fallback (the user's own already-trusted host) is
unchanged.

**Tests:** a list entry with `target = "finger://x:22/evil"` copies the `:79`
form; a `login@host` fallback entry is unaffected.

### 4. Drop stale fetch results (P2 — monotonic request ID)

**Threat.** Focus/drill keys stay live while `loading` is true, so a user can
start request A then request B before A returns. `fetchResultMsg` carries no
identity and the `Update` branch routes every result, so a late A can replace
B's view/history — with untrusted output, an old or hostile response can appear
after the user moved on.

**Change.** Monotonic request ID (chosen over context-cancellation: simpler, no
goroutine/cancel plumbing, `finger.Query` already enforces its own timeouts,
and it correctly distinguishes two fetches of the *same* target).

- Add `reqSeq uint64` to `appModel`.
- `setLoading` and `drill` increment `m.reqSeq` and pass the new value into
  `fetchCmd`.
- `fetchCmd` grows a `reqID uint64` parameter; `fetchResultMsg` grows a
  `reqID uint64` field, stamped onto the emitted message.
- The `fetchResultMsg` case in `Update` returns `m, nil` (drops the message —
  no route, no state change) when `msg.reqID != m.reqSeq`.

**Tests:** stub fetch, fire A then B, deliver A's result last, assert the view
reflects B and history did not gain A; a single in-flight request still routes
normally.

### 5. Cap parsed user lists (P2 — truncate + visible note)

**Threat.** A malicious host can return a ~1 MiB columnar/generic body with tens
of thousands of distinct short logins. `newList` allocates a `list.Item` per
parsed user and feeds them all into list/filter state, spiking memory/CPU and
freezing the TUI despite the body byte cap.

**Change.** `const maxListEntries = 2000` in `list.go` (generous vs. real host
listings of dozens–hundreds; confirmed default).

- `newList` truncates `users` to `maxListEntries` when building `items`.
- `newListWithPreamble` detects truncation (original `len(users)` vs cap) and
  prepends a note: `"list truncated — showing first 2000 of N"`, composed with
  any existing preamble / auto-detect note in the established way.

This keeps a usable, bounded list rather than declining to the raw viewport
(declining would drop the selectable UI for the very large directories where it
is most useful), and the note keeps the UI honest per the project convention.

**Tests:** feed 5000 synthetic users → `newList` yields ≤ 2000 items;
`newListWithPreamble` includes the truncation note with the correct N; a small
list (< cap) yields no note and all entries.

---

## CI

### 6. Restrict workflow token permissions (P3)

**Threat.** `ci.yml` and `vuln.yml` run repository code and fetch Go tools with
no `permissions:` block, so on a repo whose default `GITHUB_TOKEN` is read/write
the jobs get more scope than they use.

**Change.** Add a top-level `permissions:` block to both workflows:

```yaml
permissions:
  contents: read
```

No current step writes to the repo, opens issues/PRs, or publishes; additional
scopes are added later only if a step needs them. Verified by inspection (no
test).

---

## Testing & delivery

- All `finger/` and `tui/` changes are exercised by existing injected-fake
  patterns: `finger` tests against a local `net.Listen` server, `tui` tests with
  a stub `FetchFunc`. No real network or TTY is introduced.
- Gate: `make check` (vet, gofmt emptiness, golangci-lint, `go test -race`).
- Commits: Conventional Commits, **no** `Co-Authored-By` trailer (per CLAUDE.md).
  One commit per finding (or grouped per package) for reviewability.
