# v0.1.0 Security & Safety Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the six findings from the v0.1.0 security/safety review, tightening the boundary between untrusted finger responses (and server-supplied targets derived from them) and the user's terminal, clipboard, and outbound socket.

**Architecture:** Six independent fixes across three areas. `finger/` gets ingress Unicode-format-control escaping and egress query validation (pure functions, tested against a local listener). `tui/` gets clipboard target-pinning, a monotonic request-ID guard against stale fetches (via a shared `startFetch` helper), and a parsed-list size cap (tested with a stub `FetchFunc`). CI gets least-privilege token scopes.

**Tech Stack:** Go, `charm.land/bubbletea/v2` + `bubbles/v2`, standard `unicode`/`net` packages, GitHub Actions. Gate: `make check` (vet, gofmt, golangci-lint, `go test -race`). Reference spec: `docs/superpowers/specs/2026-05-31-security-hardening-design.md`.

**Conventions:** Conventional Commits, **no** `Co-Authored-By` trailer (per CLAUDE.md). Tasks are independent and may be committed separately.

---

## File map

- `finger/sanitize.go` — add Unicode Cf/Zl/Zp escaping + `writeUnicodeEscape` helper (Task 1)
- `finger/sanitize_test.go` — table cases for the above (Task 1)
- `finger/query.go` — `hasControl` helper; reject controls in `ParseTarget` and `queryWith` (Task 2)
- `finger/query_test.go` — `ParseTarget` rejection cases (Task 2)
- `finger/client_test.go` — `queryWith` backstop case (Task 2)
- `tui/app.go` — `reqSeq` field, `startFetch` helper, request-ID guard, clipboard pinning (Tasks 3, 4)
- `tui/fetch.go` — `reqID` on `fetchResultMsg`, `fetchCmd` param (Task 4)
- `tui/app_test.go` — stale-fetch and clipboard-pin tests (Tasks 3, 4)
- `tui/list.go` — `maxListEntries` cap + truncation note (Task 5)
- `tui/list_test.go` — cap and note tests (Task 5)
- `.github/workflows/ci.yml`, `.github/workflows/vuln.yml` — `permissions: contents: read` (Task 6)

---

## Task 1: Escape non-printing Unicode format controls (finger/sanitize.go)

**Files:**
- Modify: `finger/sanitize.go` (the slow-path `switch` in `sanitize`; add helper)
- Test: `finger/sanitize_test.go`

- [ ] **Step 1: Write the failing test cases**

Add these rows to the `tests` slice in `TestSanitize` (`finger/sanitize_test.go`):

```go
{"rlo override to unicode escape", "a‮b", "a\\u{202e}b"},
{"zero-width space to unicode escape", "a​b", "a\\u{200b}b"},
{"bom to unicode escape", "﻿hi", "\\u{feff}hi"},
{"zwj between emoji escaped", "\U0001f468‍\U0001f469", "\U0001f468\\u{200d}\U0001f469"},
{"line separator to unicode escape", "a b", "a\\u{2028}b"},
{"soft hyphen to unicode escape", "a­b", "a\\u{ad}b"},
{"plain emoji preserved", "hi \U0001f600", "hi \U0001f600"},
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./finger/ -run TestSanitize -count=1 -v`
Expected: FAIL — the new rows show the format runes written verbatim (e.g. `got "a‮b"`) instead of escaped.

- [ ] **Step 3: Add the escaping branch and helper**

In `finger/sanitize.go`, add `"unicode"` to the imports. In `sanitize`, insert a new case **after** the C1 case and **before** `default`:

```go
		case unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			// Non-printing Unicode format controls (bidi overrides/isolates,
			// zero-width, BOM, soft hyphen, tag chars) and line/paragraph
			// separators. These reorder or hide text without any terminal
			// escape, so visualize them rather than emit them verbatim.
			writeUnicodeEscape(&b, r)
```

Add the helper alongside `writeHex`:

```go
// writeUnicodeEscape writes a rune as lowercase `\u{XXXX}` — the notation for a
// defanged code point, distinct from writeHex's byte-oriented `\xXX`.
func writeUnicodeEscape(b *strings.Builder, r rune) {
	b.WriteString("\\u{")
	b.WriteString(strconv.FormatInt(int64(r), 16))
	b.WriteByte('}')
}
```

Add `"strconv"` to the imports.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./finger/ -run TestSanitize -count=1 -v`
Expected: PASS (all rows, including the existing C0/C1/DEL/UTF-8 rows, still green).

- [ ] **Step 5: Update the sanitize doc comment**

In the `sanitize` doc comment block listing the rules, add a bullet documenting the new behavior so the comment stays truthful:

```go
//   - non-printing Unicode format controls (category Cf — bidi overrides and
//     isolates, zero-width chars, BOM, soft hyphen, tag chars) and line/
//     paragraph separators (Zl/Zp) become lowercase "\u{XXXX}".
```

- [ ] **Step 6: Commit**

```bash
git add finger/sanitize.go finger/sanitize_test.go
git commit -m "fix(finger): escape non-printing Unicode format controls"
```

---

## Task 2: Reject control chars in the outbound query (finger/query.go)

**Files:**
- Modify: `finger/query.go` (`ParseTarget`; add `hasControl` helper), `finger/client.go` (`queryWith`)
- Test: `finger/query_test.go`, `finger/client_test.go`

- [ ] **Step 1: Write the failing ParseTarget tests**

Add to the error-case tests in `finger/query_test.go` (match the existing table/sub-test style there — find where `ParseTarget` error cases live and add rows; if cases are individual `t.Run`s, add equivalents):

```go
{name: "rejects crlf in user", arg: "a\r\nb@host", wantErr: true},
{name: "rejects nul in host", arg: "u@ho\x00st", wantErr: true},
{name: "rejects del in user", arg: "a\x7f@host", wantErr: true},
```

(If the existing `ParseTarget` test does not use a `wantErr` table, instead add three focused tests:

```go
func TestParseTargetRejectsControlChars(t *testing.T) {
	for _, arg := range []string{"a\r\nb@host", "u@ho\x00st", "a\x7f@host"} {
		if _, err := ParseTarget(arg); err == nil {
			t.Errorf("ParseTarget(%q) = nil error, want error", arg)
		}
	}
}
```

— use whichever matches the file's existing convention.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./finger/ -run TestParseTarget -count=1 -v`
Expected: FAIL — control-char args currently parse without error.

- [ ] **Step 3: Add `hasControl` and the ParseTarget guard**

In `finger/query.go`, add the helper:

```go
// hasControl reports whether s contains an ASCII C0 control (< 0x20, including
// CR and LF) or DEL (0x7f). Such bytes in a query token would let a hostile or
// malformed target smuggle extra RFC 1288 query lines onto the wire.
func hasControl(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}
```

In `ParseTarget`, after `user` and `hostport` are assigned and the empty-host check passes, add:

```go
	if hasControl(user) || hasControl(hostport) {
		return Target{}, errors.New("target contains control characters")
	}
```

(Place it just before the `return Target{...}` at the end.)

- [ ] **Step 4: Run to verify ParseTarget passes**

Run: `go test ./finger/ -run TestParseTarget -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Write the failing queryWith backstop test**

In `finger/client_test.go`, following the existing local-listener test pattern, add a test that a hand-built `Target` with a control char in `User` is refused **without** writing to the connection. Minimal version not needing a server (the guard returns before dialing if placed correctly — but the spec puts the check just before the write, so dial a local listener that records bytes):

```go
func TestQueryRejectsControlCharsInUser(t *testing.T) {
	// Listener that records anything written, so we can assert nothing was sent.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan int, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			got <- -1
			return
		}
		defer c.Close()
		buf := make([]byte, 64)
		n, _ := c.Read(buf)
		got <- n
	}()

	tgt := Target{User: "a\r\nb", HostPort: ln.Addr().String()}
	_, _, err = Query(context.Background(), tgt)
	if err == nil {
		t.Fatal("Query with control char in user = nil error, want error")
	}
}
```

(Add imports `context`, `net` if not already present in the test file.)

- [ ] **Step 6: Run to verify failure**

Run: `go test ./finger/ -run TestQueryRejectsControlChars -count=1 -v`
Expected: FAIL — `Query` currently writes the multi-line query and returns no error from the guard.

- [ ] **Step 7: Add the queryWith backstop**

In `finger/client.go`, in `queryWith`, immediately **before** the `fmt.Fprintf(conn, "%s\r\n", t.User)` write, add:

```go
	if hasControl(t.User) {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("query user contains control characters")
	}
```

- [ ] **Step 8: Run to verify pass**

Run: `go test ./finger/ -count=1 -v`
Expected: PASS (all finger tests).

- [ ] **Step 9: Commit**

```bash
git add finger/query.go finger/client.go finger/query_test.go finger/client_test.go
git commit -m "fix(finger): reject control characters in outbound query"
```

---

## Task 3: Pin copied server targets before clipboard (tui/app.go)

**Files:**
- Modify: `tui/app.go` (`copyAddress`, lines ~519-537)
- Test: `tui/app_test.go`

- [ ] **Step 1: Write the failing test**

Add to `tui/app_test.go`. Build an `appModel` in `stateList` with one server-supplied list entry pointing at a non-79 port, call `copyAddress`, and assert the flashed address is pinned to `:79`. Follow the existing app_test construction helpers; a self-contained version:

```go
func TestCopyAddressPinsServerTarget(t *testing.T) {
	host, _ := finger.ParseTarget("@dir.example")
	users := []User{{Login: "evil", Target: "finger://example.com:22/evil"}}
	common := &commonModel{width: 80, height: 24}
	m := appModel{common: common, state: stateList, list: newList(common, host, users)}
	m.list.list.Select(0)

	_ = m.copyAddress()

	if !strings.Contains(m.flash, ":79") || strings.Contains(m.flash, ":22") {
		t.Fatalf("copyAddress flash = %q, want pinned to :79", m.flash)
	}
}
```

(Adjust struct field names to match the real `appModel`/`commonModel` if the file uses constructors; check an existing app_test for the canonical setup.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tui/ -run TestCopyAddressPinsServerTarget -count=1 -v`
Expected: FAIL — flash contains `:22` (raw target copied).

- [ ] **Step 3: Pin the target in copyAddress**

In `tui/app.go` `copyAddress`, replace the server-supplied branch so it parses and pins, mirroring `drill`:

```go
	if m.state == stateList {
		if sel, ok := m.list.selected(); ok {
			if sel.target != "" {
				// Mirror drill's safety: a server-supplied target could point at
				// an arbitrary host:port. Pin to finger's port 79 before copying
				// so a pasted-back address can't be steered at another service.
				if t, err := finger.ParseTarget(sel.target); err == nil {
					addr = rawFromTarget(pinFingerPort(t))
				}
			} else {
				addr = sel.login + "@" + strings.TrimPrefix(m.list.host.Raw, "@")
			}
		}
	} else if m.pos >= 0 {
```

(Leave the rest of `copyAddress` unchanged.)

- [ ] **Step 4: Run to verify pass**

Run: `go test ./tui/ -run TestCopyAddressPinsServerTarget -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "fix(tui): pin server-supplied targets before copying to clipboard"
```

---

## Task 4: Drop stale fetch results via monotonic request ID (tui/app.go, tui/fetch.go)

**Files:**
- Modify: `tui/fetch.go` (`fetchResultMsg`, `fetchCmd`), `tui/app.go` (`reqSeq` field, `startFetch` helper, the two fetch-start sites, the `fetchResultMsg` case)
- Test: `tui/app_test.go`

- [ ] **Step 1: Write the failing test**

Add to `tui/app_test.go`. Use a stub `FetchFunc` whose responses you control, fire request A then B, deliver A's result *after* B's, and assert the view reflects B. Concrete approach using the existing fetch-stub style: drive `Update` with two `fetchResultMsg`s out of order and assert the stale one is dropped.

```go
func TestStaleFetchResultDropped(t *testing.T) {
	common := &commonModel{width: 80, height: 24}
	m := appModel{common: common}

	// Simulate two in-flight requests: reqSeq advances to 2 after B starts.
	m.reqSeq = 2
	m.loading = true

	// A late result stamped with the *old* id (1) must be dropped: no route.
	stale := fetchResultMsg{reqID: 1, entry: Entry{Target: finger.Target{Raw: "a@x"}}}
	updated, _ := m.Update(stale)
	got := updated.(appModel)
	if got.pos != m.pos {
		t.Fatalf("stale result mutated history: pos %d -> %d", m.pos, got.pos)
	}
	if !got.loading {
		t.Fatal("stale result cleared loading; in-flight request B should still be loading")
	}

	// The current result (id 2) routes normally and clears loading.
	current := fetchResultMsg{reqID: 2, entry: Entry{Target: finger.Target{Raw: "b@x"}, Body: []byte("hi\n")}}
	updated, _ = got.Update(current)
	if updated.(appModel).loading {
		t.Fatal("current result did not clear loading")
	}
}
```

(`m.pos` starts at the appModel zero value; confirm the field name for the history cursor — the codebase uses `m.pos`. Adjust if construction needs `m.history` initialized.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tui/ -run TestStaleFetchResultDropped -count=1 -v`
Expected: FAIL to compile — `fetchResultMsg` has no `reqID` field; and once that's added, the routing-every-result behavior would clear loading on the stale message.

- [ ] **Step 3: Add reqID to the fetch plumbing**

In `tui/fetch.go`, add the field and parameter:

```go
type fetchResultMsg struct {
	reqID uint64
	entry Entry
}
```

```go
func fetchCmd(ctx context.Context, fetch FetchFunc, target finger.Target, reqID uint64) tea.Cmd {
	return func() tea.Msg {
		body, meta, err := fetch(ctx, target)
		return fetchResultMsg{
			reqID: reqID,
			entry: Entry{
				Target: target,
				Body:   body,
				Meta:   meta,
				Err:    err,
			},
		}
	}
}
```

- [ ] **Step 4: Add the reqSeq field and startFetch helper**

In `tui/app.go`, add to the `appModel` struct (near `loading bool` / `loadingTarget`):

```go
	reqSeq uint64 // monotonic id of the most recently started fetch
```

Add a helper that centralizes the (previously duplicated) fetch-start sequence and stamps the id:

```go
// startFetch marks loading for target, advances the request id so any
// still-in-flight earlier fetch's result will be discarded on arrival, and
// returns the command batch that performs the fetch and ticks the spinner.
func (m *appModel) startFetch(target finger.Target) tea.Cmd {
	m.loading = true
	m.loadingTarget = target
	m.reqSeq++
	return tea.Batch(fetchCmd(context.Background(), m.common.fetch, target, m.reqSeq), m.spin.Tick)
}
```

- [ ] **Step 5: Route both fetch-start sites through startFetch**

In the submit path (`tui/app.go` ~line 265-267), replace:

```go
	m.loading = true
	m.loadingTarget = target
	return tea.Batch(fetchCmd(context.Background(), m.common.fetch, target), m.spin.Tick)
```

with:

```go
	return m.startFetch(target)
```

In `drill` (`tui/app.go` ~line 446-451), replace:

```go
	m.loading = true
	m.loadingTarget = target
	// Keep the current view (the list) on screen while loading; routeFetch sets
	// the final state when the result lands. Switching to the reader eagerly here
	// flashed the previous profile for a frame before the new one arrived.
	return true, m, tea.Batch(fetchCmd(context.Background(), m.common.fetch, target), m.spin.Tick)
```

with:

```go
	// Keep the current view (the list) on screen while loading; routeFetch sets
	// the final state when the result lands. Switching to the reader eagerly here
	// flashed the previous profile for a frame before the new one arrived.
	cmd := m.startFetch(target)
	return true, m, cmd
```

**Important — evaluation order:** `drill` has a value receiver and returns `m` by value. You MUST assign `cmd` on its own line first. Writing `return true, m, m.startFetch(target)` would evaluate the returned `m` (2nd value) *before* `startFetch` mutates it (3rd value), so `loading`/`reqSeq` would be lost from the returned model. The two-line form above takes `&m` for the pointer-receiver call, mutates the local `m`, then returns that mutated value. The submit-path site can keep the one-line `return m.startFetch(target)` form because it returns only the `tea.Cmd` (its receiver is `*appModel`, so the mutation persists through the pointer) — but if that method turns out to have a value receiver, switch it to the same two-line form and ensure the caller adopts the returned model.

- [ ] **Step 6: Add the request-ID guard to the fetchResultMsg case**

In `tui/app.go` `Update`, replace:

```go
	case fetchResultMsg:
		return m.routeFetch(msg.entry), nil
```

with:

```go
	case fetchResultMsg:
		if msg.reqID != m.reqSeq {
			// A superseded in-flight fetch finished late; drop it so it cannot
			// replace the current view/history with stale (or hostile) output.
			return m, nil
		}
		return m.routeFetch(msg.entry), nil
```

- [ ] **Step 7: Run to verify pass**

Run: `go test ./tui/ -run TestStaleFetchResultDropped -count=1 -v`
Expected: PASS.

- [ ] **Step 8: Run the full tui suite (guards against missed call sites)**

Run: `go test ./tui/ -count=1`
Expected: PASS — confirms every `fetchCmd` caller now passes the id and no other site set `loading` directly in a way the guard breaks.

- [ ] **Step 9: Commit**

```bash
git add tui/app.go tui/fetch.go tui/app_test.go
git commit -m "fix(tui): drop stale fetch results via monotonic request id"
```

---

## Task 5: Cap parsed user lists (tui/list.go)

**Files:**
- Modify: `tui/list.go` (`newList`, `newListWithPreamble`)
- Test: `tui/list_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `tui/list_test.go`:

```go
func TestNewListCapsEntries(t *testing.T) {
	users := make([]User, 5000)
	for i := range users {
		users[i] = User{Login: fmt.Sprintf("u%d", i)}
	}
	common := &commonModel{width: 80, height: 24}
	m := newList(common, finger.Target{Raw: "@big.example"}, users)
	if got := len(m.list.Items()); got > maxListEntries {
		t.Fatalf("newList kept %d items, want <= %d", got, maxListEntries)
	}
}

func TestNewListWithPreambleNotesTruncation(t *testing.T) {
	users := make([]User, 5000)
	for i := range users {
		users[i] = User{Login: fmt.Sprintf("u%d", i)}
	}
	common := &commonModel{width: 80, height: 24}
	m := newListWithPreamble(common, finger.Target{Raw: "@big.example"}, users, nil, false)
	if !strings.Contains(m.preamble, "truncated") {
		t.Fatalf("preamble = %q, want a truncation note", m.preamble)
	}
}
```

(Add `"fmt"` to the test imports if absent.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tui/ -run 'TestNewList' -count=1 -v`
Expected: FAIL to compile — `maxListEntries` undefined.

- [ ] **Step 3: Add the cap to newList**

In `tui/list.go`, add the constant near the other list constants (top of file or beside `listChromeRows`):

```go
// maxListEntries bounds how many parsed users we turn into Bubble list items.
// A hostile host can pack a 1 MiB response with tens of thousands of distinct
// logins; capping keeps list/filter state bounded so the TUI can't be frozen.
const maxListEntries = 2000
```

In `newList`, cap before allocating items:

```go
func newList(common *commonModel, host finger.Target, users []User) listModel {
	if len(users) > maxListEntries {
		users = users[:maxListEntries]
	}
	items := make([]list.Item, len(users))
	for i, u := range users {
		items[i] = userItem{login: u.Login, name: u.Name, target: u.Target}
	}
	// ... unchanged ...
```

- [ ] **Step 4: Add the truncation note to newListWithPreamble**

`newList` truncates internally, so compute truncation from the original length in `newListWithPreamble` before calling `newList`:

```go
func newListWithPreamble(common *commonModel, host finger.Target, users []User, body []byte, generic bool) listModel {
	total := len(users)
	m := newList(common, host, users)
	m.generic = generic
	if parsed, ok := parseUserList(body); ok {
		m.preamble = parsed.preamble
	} else {
		m.preamble = extractListPreamble(body)
	}
	if total > maxListEntries {
		note := fmt.Sprintf("list truncated — showing first %d of %d", maxListEntries, total)
		if m.preamble != "" {
			m.preamble = note + "\n\n" + m.preamble
		} else {
			m.preamble = note
		}
	}
	if generic {
		note := "Auto-detected user list from an unrecognized response — press r to view raw."
		if m.preamble != "" {
			m.preamble = note + "\n\n" + m.preamble
		} else {
			m.preamble = note
		}
	}
	m.setSize(common.width, common.bodyHeight())
	return m
}
```

(Confirm `fmt` is imported in `list.go` — it already is, used by `l.Title`.)

- [ ] **Step 5: Run to verify pass**

Run: `go test ./tui/ -run 'TestNewList' -count=1 -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tui/list.go tui/list_test.go
git commit -m "fix(tui): cap parsed user lists to bound list state"
```

---

## Task 6: Restrict CI workflow token permissions (.github/workflows)

**Files:**
- Modify: `.github/workflows/ci.yml`, `.github/workflows/vuln.yml`

- [ ] **Step 1: Add permissions to ci.yml**

In `.github/workflows/ci.yml`, insert a top-level `permissions` block between the `on:` block and `jobs:`:

```yaml
on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
```

- [ ] **Step 2: Add permissions to vuln.yml**

In `.github/workflows/vuln.yml`, insert the same block between the `on:` block and `jobs:`:

```yaml
  schedule:
    - cron: "0 7 * * 1" # Mondays 07:00 UTC

permissions:
  contents: read

jobs:
```

- [ ] **Step 3: Verify YAML validity by inspection**

Run: `git diff .github/workflows/`
Expected: each file gains exactly the three-line `permissions:` block at top level (not nested under a job). No step in either workflow writes to the repo, opens issues/PRs, or publishes, so `contents: read` is sufficient.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/vuln.yml
git commit -m "ci: restrict workflow token permissions to contents: read"
```

---

## Final verification

- [ ] **Run the full gate set**

Run: `make check`
Expected: PASS — `go vet`, gofmt emptiness, `golangci-lint run ./...`, and `go test ./... -race` all green.

- [ ] **Confirm the spec is fully covered**

Each finding (1–6) maps to Task 1–6 respectively; no spec section is left unimplemented.
