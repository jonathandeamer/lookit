# RFC 1288 §3.3 control-character filtering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Defang control/escape data in finger response bodies at ingress (in `finger.Query`), visualizing offending bytes rather than deleting them, so a hostile or garbled server cannot inject escape sequences into the user's terminal.

**Architecture:** A new pure `sanitize([]byte) []byte` in `finger/` is called inside `queryWith` immediately after the existing `CRLF→LF` normalization and before `meta.Bytes`/truncation are computed. It walks the body rune by rune: keeps tab, newline, and all printable runes (including valid multibyte UTF-8); rewrites C0 controls (except tab/newline) and DEL to caret notation (`^[`, `^?`); rewrites C1 controls (U+0080–U+009F, even when validly UTF-8-encoded) and any invalid UTF-8 byte to `\xXX` hex. Every downstream consumer (CLI render, TUI reader, TUI list delegate, `r` raw view, `y`-copy) receives already-safe bytes. `render/`, `ParseUsers`, and the TUI are untouched in their logic.

**Tech Stack:** Go, standard library only (`unicode/utf8`, `strings`/`bytes`). Tests use the existing `finger` `fakeServer` local-listener harness and the `render` golden corpus; all offline.

**Spec:** `docs/superpowers/specs/2026-05-31-rfc1288-control-char-filtering-design.md`
**Conformance record to flip:** `docs/rfc1288-conformance.md` (§3.3 rows 🔶 → ✅)

---

## File Structure

- **Create `finger/sanitize.go`** — the pure `sanitize([]byte) []byte` function and its rune-classification helpers. One responsibility: turn arbitrary response bytes into terminal-safe bytes. No I/O, no other dependencies.
- **Create `finger/sanitize_test.go`** — table-driven unit tests for `sanitize` in isolation.
- **Modify `finger/client.go`** — call `sanitize` in `queryWith` after the CRLF→LF step (one line) and add one integration test to the existing `finger` test suite proving an ESC-bearing server response is defanged end-to-end.
- **Modify `docs/rfc1288-conformance.md`** — flip the §3.3 status from "specced, not yet implemented" (🔶) to met (✅) once the code lands.
- **`render/`, `tui/`** — no logic changes; their existing tests serve as regression guards (render golden corpus must stay byte-identical; a new TUI test proves the list-delegate hole is closed).

A note on test placement: the `finger` package's integration test helper `fakeServer` lives in `finger/client_test.go`. Add the integration test (Task 3) to that file so it reuses the helper without redeclaring it.

---

### Task 1: The `sanitize` pure function

**Files:**
- Create: `finger/sanitize.go`
- Test: `finger/sanitize_test.go`

- [ ] **Step 1: Write the failing tests**

Create `finger/sanitize_test.go`:

```go
package finger

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "clean profile is unchanged",
			in:   "Login: alice\nName: Alice Example\nPlan:\nhello\n",
			want: "Login: alice\nName: Alice Example\nPlan:\nhello\n",
		},
		{
			name: "tab and newline preserved",
			in:   "a\tb\nc\n",
			want: "a\tb\nc\n",
		},
		{
			name: "ESC becomes caret notation",
			in:   "before\x1b[31mred\x1b[0mafter",
			want: "before^[[31mred^[[0mafter",
		},
		{
			name: "BEL becomes caret G",
			in:   "ding\x07dong",
			want: "ding^Gdong",
		},
		{
			name: "NUL becomes caret at-sign",
			in:   "a\x00b",
			want: "a^@b",
		},
		{
			name: "unit separator becomes caret underscore",
			in:   "a\x1fb",
			want: "a^_b",
		},
		{
			name: "DEL becomes caret question",
			in:   "a\x7fb",
			want: "a^?b",
		},
		{
			name: "stray carriage return becomes caret M",
			in:   "a\rb",
			want: "a^Mb",
		},
		{
			name: "valid multibyte UTF-8 preserved",
			in:   "café — 日本語 — 🎉 — ┌─┐",
			want: "café — 日本語 — 🎉 — ┌─┐",
		},
		{
			name: "raw high C1 byte becomes hex",
			in:   "a\x9bb",
			want: `a\x9bb`,
		},
		{
			name: "UTF-8-encoded C1 (U+0085) becomes hex",
			in:   "ab", // bytes 0xc2 0x85
			want: `a\x85b`,
		},
		{
			name: "invalid UTF-8 byte becomes hex",
			in:   "a\xffb",
			want: `a\xffb`,
		},
		{
			name: "empty input",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(sanitize([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./finger/ -run TestSanitize -count=1 -v`
Expected: FAIL to compile — `undefined: sanitize`.

- [ ] **Step 3: Write the implementation**

Create `finger/sanitize.go`:

```go
package finger

import (
	"strings"
	"unicode/utf8"
)

// sanitize makes a finger response body safe to print to a terminal, per
// RFC 1288 §3.3 ("filter any unprintable data"). It visualizes rather than
// deletes control data, so no information is lost:
//
//   - tab, newline, and every printable rune (including all valid multibyte
//     UTF-8) are kept verbatim;
//   - C0 controls (U+0000–U+001F except tab/newline) and DEL (U+007F) become
//     caret notation (ESC -> "^[", BEL -> "^G", DEL -> "^?");
//   - C1 controls (U+0080–U+009F, even when validly UTF-8-encoded) and any
//     invalid UTF-8 byte become lowercase "\xXX" hex.
//
// We deliberately keep UTF-8 rather than honoring §3.3's literal "7-bit"
// wording: stripping bytes >= 0x80 would delete legitimate modern content,
// while the genuine terminal-control threat is the control ranges above.
func sanitize(body []byte) []byte {
	// Fast path: if there is nothing to defang, return the input unchanged.
	if isClean(body) {
		return body
	}
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRune(body[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8 byte.
			writeHex(&b, body[i])
			i++
			continue
		}
		switch {
		case r == '\t' || r == '\n':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			// C0 control (except tab/newline already handled) or DEL.
			writeCaret(&b, r)
		case r >= 0x80 && r <= 0x9f:
			// C1 control, however it was encoded.
			writeHex(&b, byte(r))
		default:
			b.WriteRune(r)
		}
		i += size
	}
	return []byte(b.String())
}

// isClean reports whether body contains only bytes sanitize would keep
// verbatim, allowing the common case to skip allocation. It is conservative:
// any byte that could need defanging (control, DEL, or >= 0x80) forces the
// slow path, where DecodeRune does the precise classification.
func isClean(body []byte) bool {
	for _, c := range body {
		if c == '\t' || c == '\n' {
			continue
		}
		if c < 0x20 || c >= 0x7f {
			return false
		}
	}
	return true
}

// writeCaret writes a C0 control or DEL in caret notation: the control's
// printable counterpart is the code point XOR 0x40 (NUL -> '@', US -> '_',
// DEL -> '?').
func writeCaret(b *strings.Builder, r rune) {
	b.WriteByte('^')
	b.WriteByte(byte(r) ^ 0x40)
}

// writeHex writes a single byte as lowercase `\xXX`.
func writeHex(b *strings.Builder, c byte) {
	const hex = "0123456789abcdef"
	b.WriteByte('\\')
	b.WriteByte('x')
	b.WriteByte(hex[c>>4])
	b.WriteByte(hex[c&0xf])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./finger/ -run TestSanitize -count=1 -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add finger/sanitize.go finger/sanitize_test.go
git commit -m "feat(finger): add control-char sanitizer for RFC 1288 §3.3"
```

---

### Task 2: Wire `sanitize` into `queryWith`

**Files:**
- Modify: `finger/client.go` (the `CRLF→LF` line in `queryWith`, currently around line 88)

- [ ] **Step 1: Make the change**

In `finger/client.go`, find this line in `queryWith`:

```go
	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	meta.Bytes = len(body)
```

Insert the `sanitize` call between them so byte count and truncation logic see the final, user-visible form:

```go
	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	body = sanitize(body)
	meta.Bytes = len(body)
```

Leave everything else (the `truncatedByCap` handling, the `readErr` branch, and the trailing-`\n` truncation check) unchanged — sanitize never alters `\n`, so the "last byte is newline?" test in the `readErr` branch still behaves correctly.

- [ ] **Step 2: Run the full finger suite to verify nothing regressed**

Run: `go test ./finger/ -count=1`
Expected: PASS. In particular `TestQuery_Success` (which asserts a clean `Login:/Name:` body round-trips to `"Login: alice\nName: Alice\n"`) still passes, because `sanitize` is a no-op on clean bodies.

- [ ] **Step 3: Commit**

```bash
git add finger/client.go
git commit -m "feat(finger): sanitize response body at ingress in Query"
```

---

### Task 3: Integration test — ESC-bearing server response is defanged end-to-end

**Files:**
- Modify: `finger/client_test.go` (add one test; reuses the existing `fakeServer` helper defined in that file)

- [ ] **Step 1: Write the failing test**

Append to `finger/client_test.go`:

```go
func TestQuery_DefangsControlBytes(t *testing.T) {
	// Server sends a body containing an ESC sequence and a BEL, ending in CRLF.
	resp := []byte("Name: \x1b[31mEvil\x1b[0m\x07\r\n")
	addr := fakeServer(t, resp, 0)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	target, err := ParseTarget(host + ":" + port)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	body, meta, err := Query(context.Background(), target)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	got := string(body)
	want := "Name: ^[[31mEvil^[[0m^G\n"
	if got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	// No live ESC may survive into the body handed to callers.
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("body still contains a raw ESC: %q", got)
	}
	// meta.Bytes reflects the post-sanitize length.
	if meta.Bytes != len(body) {
		t.Errorf("meta.Bytes = %d, want %d", meta.Bytes, len(body))
	}
	// Body ends in newline, so it must not be marked truncated.
	if meta.Truncated {
		t.Errorf("meta.Truncated = true, want false")
	}
}
```

(If `strings` is not already imported in `client_test.go`, add it to the import block — it is used by existing tests there, so it most likely already is.)

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./finger/ -run TestQuery_DefangsControlBytes -count=1 -v`
Expected: PASS. (Tasks 1–2 already implement the behavior; this test guards the end-to-end wiring through `Query`.)

- [ ] **Step 3: Commit**

```bash
git add finger/client_test.go
git commit -m "test(finger): cover end-to-end defanging of control bytes in Query"
```

---

### Task 4: Regression guard — render golden corpus is byte-identical

**Files:**
- No source changes. Run the existing `render` tests.

- [ ] **Step 1: Run the render suite without -update**

Run: `go test ./render/ -count=1 -v`
Expected: PASS with **no** golden diffs. In particular `TestRender_AsciiArtPreserved` still passes, because the `ascii-art` capture contains no control bytes, so `sanitize` upstream would have been a no-op anyway — and `render` itself is unchanged.

Note: this task changes nothing; it exists so the executor explicitly confirms the render path is untouched. If any golden file differs, STOP — that means `sanitize` is altering printable content, which is a bug in Task 1, not a reason to run `-update`.

- [ ] **Step 2: No commit** (nothing changed). Proceed.

---

### Task 5: TUI test — the list-delegate hole is closed

**Files:**
- Test: add to `tui/userlist_test.go` (pure-parser level — the safest, dependency-free place to prove a parsed `Name`/display field carries no live ESC once the body is sanitized).

Context: the spec calls out the TUI list delegate (`userItem.Description()` = a name parsed from the body) as a path that renders *outside* the `render` package, which is why filtering at ingress (not in `render`) matters. The body reaching `ParseUsers` in production has already passed through `sanitize`. This test encodes that contract at the parser boundary: feed `ParseUsers` an already-sanitized body and assert no parsed field contains a raw ESC.

- [ ] **Step 1: Inspect how `ParseUsers` is called and what it returns**

Run: `grep -n "func ParseUsers" tui/userlist.go`
Run: `grep -n "ParseUsers(" tui/userlist_test.go | head`
Read the surrounding test(s) to match the exact call signature and the `User` field names (e.g. `Login`, `Name`, `Target`) used in assertions. Use those exact names in Step 2 rather than guessing.

- [ ] **Step 2: Write the test**

Add to `tui/userlist_test.go` (adjust the `ParseUsers` call and field access to match what Step 1 found — this example assumes `users, _, ok := ParseUsers(body)` returning a slice of `User` with a `Name` field; if the real signature differs, mirror the neighboring tests):

```go
func TestParseUsers_NoLiveEscapeInParsedFields(t *testing.T) {
	// A host user list whose display column already passed through finger
	// ingress sanitization (ESC -> "^["). ParseUsers must never resurrect a
	// raw ESC into a field the list delegate will render.
	body := []byte("Login     Name\n" +
		"alice     ^[[31mAlice^[[0m\n" +
		"bob       Bob\n")

	parsed, ok := ParseUsers(body) // match real signature from Step 1
	if !ok {
		t.Fatalf("ParseUsers declined a columnar list it should accept")
	}
	for _, u := range parsed {
		for _, field := range []string{u.Login, u.Name, u.Target} { // match real fields
			if strings.ContainsRune(field, 0x1b) {
				t.Errorf("parsed field contains a raw ESC: %q", field)
			}
		}
	}
}
```

Ensure `strings` is imported in `tui/userlist_test.go` (add it if Step 1 showed it is not already present).

- [ ] **Step 3: Run the test**

Run: `go test ./tui/ -run TestParseUsers_NoLiveEscapeInParsedFields -count=1 -v`
Expected: PASS. `ParseUsers` does not introduce ESC bytes; it only slices the already-safe input. If the chosen body is declined by every matcher, adjust it to a shape an existing matcher accepts (consult the columnar/grid cases already in `userlist_test.go`) — the point is that whatever it parses contains no live ESC.

- [ ] **Step 4: Run the whole tui suite to confirm no regression**

Run: `go test ./tui/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tui/userlist_test.go
git commit -m "test(tui): assert parsed user fields carry no live escape sequences"
```

---

### Task 6: Flip the conformance record to met

**Files:**
- Modify: `docs/rfc1288-conformance.md`

- [ ] **Step 1: Update the §3.3 rows and surrounding prose**

Make these edits in `docs/rfc1288-conformance.md`:

1. The §3.3 **filter** row — change the Status cell from:
   `🔶 **Specced, not yet implemented.** Will be met by ingress-time control-character filtering — see "§3.3 resolution" below. We intentionally depart from the literal "7-bit" wording to preserve UTF-8 (documented there).`
   to:
   `✅ Met by ingress-time control-character filtering in finger.Query (sanitize, `finger/sanitize.go`). We intentionally depart from the literal "7-bit" wording to preserve UTF-8 — see "§3.3 resolution" below.`

2. The §3.3 **toggle** row — change `🔶 *Considered, declined* (the spec records this decision)` to `✅ *Considered, declined*` and change "the planned visualize-not-delete approach" to "the visualize-not-delete approach".

3. The §3.1 row — change `… and reset-after-body handled (`client.go`). Control-char sanitization (§3.3) will add to this once implemented.` to `… reset-after-body handled, and control-char sanitization (`finger/sanitize.go`).`

4. The §2.2 MUST row — change `defensively, the response side now also hex-escapes any non-ASCII/invalid bytes (see §3.3).` (it already reads in the present tense as a planned defense — confirm it now reads as implemented; if it still hedges, make it state the response body is sanitized).

5. The **SHOULDs Result** paragraph — change `one specced and pending implementation, one considered-and-declined with reason` to `one implemented, one considered-and-declined with reason`.

6. Delete the blockquote note that begins `> **Note:** §3.3 filtering is specified in … but **not yet implemented in code.**` (the row is now ✅).

7. The **§3.3 resolution** heading body — change `**What we will do** (per the spec; not yet in code). finger.Query sanitizes …` back to `**What we do.** finger.Query sanitizes …`.

8. The **Legend** — it is fine to leave the 🔶 definition as-is (the deferred MAYs still use it). No change required.

- [ ] **Step 2: Verify no stale "not yet implemented" / "will" language remains for §3.3**

Run: `grep -n "not yet implemented\|will be met\|What we will do\|pending implementation" docs/rfc1288-conformance.md`
Expected: no matches referring to §3.3 (the deferred-MAY rows for `/W` and `{Q2}` may legitimately still say "deferred", which is different — those are fine).

- [ ] **Step 3: Commit**

```bash
git add docs/rfc1288-conformance.md
git commit -m "docs: mark RFC 1288 §3.3 control-char filtering as implemented"
```

---

### Task 7: Full gate check

**Files:** none.

- [ ] **Step 1: Run the full CI gate set**

Run: `make check`
Expected: PASS — all four gates green (`go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`, `go test ./... -race`).

If `gofmt -l .` lists `finger/sanitize.go` or any test file, run `make fmt` and re-run `make check`, then amend the relevant commit or add a `style:` fixup commit.

- [ ] **Step 2: Run the vulnerability scan** (cheap, and part of CI)

Run: `make vuln`
Expected: PASS (no new findings; the change adds only stdlib usage).

- [ ] **Step 3: No code commit** unless `make fmt` changed files; if it did:

```bash
git add -A
git commit -m "style: gofmt sanitizer"
```

---

## Self-Review

**Spec coverage** (each spec section → task):
- "The sanitizer" (rune-walk; keep tab/newline/printable/UTF-8; C0+DEL→caret; C1+invalid→hex) → Task 1 (function + unit tests covering every class incl. UTF-8-encoded C1 and invalid byte).
- "Why ingress, not the render path" / wiring after CRLF→LF, before meta.Bytes/truncation → Task 2.
- "What does not change → meta.Truncated logic" → asserted in Task 3 (ends-in-newline → not truncated) and Task 2 note.
- "What does not change → render/ untouched, ascii-art golden byte-identical" → Task 4 (explicit no-update regression).
- "What does not change → ParseUsers/corpus green" → Task 5 Step 4 (whole tui suite) + Task 7 (`go test ./...`).
- "What this closes → TUI list delegate path" → Task 5 (parser-boundary test proving no live ESC in parsed fields).
- "No toggle" → no task needed (it's the absence of a feature); recorded in the conformance doc, unchanged here.
- "CI gates" → Task 7.
- Conformance doc flip 🔶→✅ (requested by user) → Task 6.

**Placeholder scan:** No TBD/TODO/"handle edge cases"/"similar to". All code steps show complete code. Task 5's body/signature is explicitly gated on a Step-1 inspection because the exact `ParseUsers` signature/field names live in `tui/userlist.go`; the test code is complete modulo those names, with instructions to mirror neighboring tests.

**Type consistency:** `sanitize([]byte) []byte`, `isClean([]byte) bool`, `writeCaret(*strings.Builder, rune)`, `writeHex(*strings.Builder, byte)` are used consistently across Task 1 (definition) and Task 2 (single call site `body = sanitize(body)`). `meta.Bytes` and `meta.Truncated` match the existing `Meta` struct fields used in `client.go` and `client_test.go`.

**One known unknown, intentionally deferred to execution:** the exact `ParseUsers` return signature and `User` field names (Task 5) — resolved by inspection in Task 5 Step 1 rather than guessed here, because guessing a wrong field name would be a plan failure worse than an explicit lookup instruction.
