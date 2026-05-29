# Generic token-selection fallback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a last-resort `parseGenericList` to the TUI so host responses that no named parser recognizes still open a selectable list when the body carries a confident, structurally list-like signal — flagged `(best guess)` with a "view raw" escape hatch.

**Architecture:** A pure `parseGenericList(lines)` becomes the final branch of `parseUserList`, after `parseTelehackStatus`. It opens a list only on ≥2 *structured login lines* (bare login, or login + tab/2-space columnar gap — never colon or single-space). It additively harvests `finger://` / `finger user@host` drill targets via the existing `fingerURLRe`/`fingerCommandRe`, which can never open a list on their own. A `generic` flag threads through `routeFetch` to add a `(best guess)` title suffix, a preamble note, and bind `r` (view cached raw body) on generic lists only.

**Tech Stack:** Go; Bubble Tea v2 (`charm.land/...`); existing `tui` package fakes (`FetchFunc`, stub fetch). No new dependencies, no network in tests.

**Spec:** `docs/superpowers/specs/2026-05-29-generic-list-fallback-design.md`

**Conventions reminder:** Conventional Commits. **Do NOT add `Co-Authored-By` or any trailer** — this project omits them. Before committing, the CI gates are `go vet ./...`, `gofmt -l .` (must print nothing; run `gofmt -w .` to fix), and `go test ./... -race`.

---

## File structure

- **Modify `tui/userlist.go`** — add `generic bool` to `parsedUserList`; add `structuredLogin`, `parseGenericList`, `appendHarvestedTargets`; wire `parseGenericList` into `parseUserList`.
- **Modify `tui/userlist_test.go`** — parse + decline corpus for the generic fallback.
- **Modify `tui/list.go`** — add `generic bool` to `listModel`; extend `newListWithPreamble` with a `generic` param (title suffix + preamble note).
- **Modify `tui/list_test.go`** — flag/note assertions.
- **Modify `tui/app.go`** — `routeFetch` passes `parsed.generic`; `handleKey` binds `r` on a generic list.
- **Modify `tui/app_test.go`** — generic-opens-list + escape-hatch model tests; update the existing `newListWithPreamble` call site.

---

## Task 1: Structured-login gate (`parseGenericList` core)

**Files:**
- Modify: `tui/userlist.go`
- Test: `tui/userlist_test.go`

- [ ] **Step 1: Write failing tests for the structured-login gate**

Add to `tui/userlist_test.go`:

```go
// --- Generic fallback: structured-login gate ---

func TestParseGenericBareLoginBlock(t *testing.T) {
	// No Login header, no online/logged-in cue, no "> " marker, no named menu:
	// every earlier matcher declines, so the generic fallback must open this.
	body := []byte("the crew:\nbetsy\nMelchizedek\nOleander\nStarbloom\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true (bare-login block)")
	}
	want := []string{"betsy", "Melchizedek", "Oleander", "Starbloom"}
	if got := logins(users); !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
}

func TestParseGenericColumnarNoHeader(t *testing.T) {
	// login + 2-space gap + name, but no "Login" header so parseColumnar declines.
	body := []byte("klu      pilot\ntomasino  navigator\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true (headerless columnar)")
	}
	if got, want := logins(users), []string{"klu", "tomasino"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v", got, want)
	}
	if users[0].Name != "pilot" || users[1].Name != "navigator" {
		t.Fatalf("names = %q,%q want pilot,navigator", users[0].Name, users[1].Name)
	}
}

func TestGenericRequiresTwoLogins(t *testing.T) {
	// A single bare-login line is not enough to open a list.
	if _, ok := ParseUsers([]byte("Welcome.\n\nbetsy\n")); ok {
		t.Fatal("ParseUsers ok = true, want false (only one structured login)")
	}
}

func TestGenericDeclinesColonLegendDebian(t *testing.T) {
	// db.debian.org daemon help: a "key : value" attribute legend must NOT be
	// read as a user list. This is the headline guard for excluding the colon
	// (and single-space) form from the generic fallback.
	body := []byte("userdir-ldap finger daemon\n" +
		"--------------------------\n" +
		"finger <uid>[/<attributes>]@db.debian.org\n" +
		"    The following attributes are currently supported:\n" +
		"      cn : First name\n" +
		"      mn : Middle name\n" +
		"      sn : Last name\n" +
		"      email : Email\n" +
		"      key : Key block\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (colon attribute legend)")
	}
}

func TestGenericDeclinesSingleSpaceProse(t *testing.T) {
	// "login value" with a single space is prose, not a columnar entry.
	body := []byte("must provide username\nplease try again later\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (single-space prose)")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestParseGeneric|TestGeneric' -count=1 -v`
Expected: FAIL — `ParseUsers` declines the two parse cases (`parseGenericList` does not exist yet, so `parseUserList` returns false for them).

- [ ] **Step 3: Add the `generic` field to `parsedUserList`**

In `tui/userlist.go`, extend the struct:

```go
type parsedUserList struct {
	users    []User
	preamble string
	generic  bool
}
```

- [ ] **Step 4: Wire `parseGenericList` in as the final branch**

In `parseUserList`, immediately before the final `return parsedUserList{}, false`:

```go
	if users, preamble, ok := parseGenericList(lines); ok {
		return parsedUserList{users: users, preamble: preamble, generic: true}, true
	}
	return parsedUserList{}, false
```

- [ ] **Step 5: Implement `structuredLogin` and `parseGenericList`**

Append to `tui/userlist.go` (the target-harvesting call is added in Task 2; for now `parseGenericList` returns the structured entries only):

```go
// structuredLogin reports whether a single line is a generic structured login
// entry, returning the login and a best-effort name. It accepts only two
// shapes: a bare login (the whole trimmed line is one loginRe token), or a
// columnar login (first token is a loginRe token followed by a tab or 2+
// spaces, then free text taken as the name). A login followed by a single
// space, and any "login : value" colon form, are treated as prose and rejected
// — those shapes appear constantly in help text, legends, and glossaries
// (e.g. db.debian.org's "cn : First name"), where they are not user lists.
func structuredLogin(line string) (login, name string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 1 {
		if loginRe.MatchString(fields[0]) {
			return fields[0], "", true
		}
		return "", "", false
	}
	first := fields[0]
	if !loginRe.MatchString(first) {
		return "", "", false
	}
	// trimmed begins with first (leading space already removed); the gap that
	// follows must be a tab or 2+ spaces to count as a deliberate column layout.
	rest := trimmed[len(first):]
	if strings.HasPrefix(rest, "\t") || strings.HasPrefix(rest, "  ") {
		return first, strings.TrimSpace(rest), true
	}
	return "", "", false
}

// parseGenericList is the last-resort matcher, tried only after every named
// parser declines. It finds the longest contiguous run of structuredLogin
// lines and opens a list when that run holds >= 2 distinct logins; otherwise it
// declines. A blank or non-entry line ends a run.
func parseGenericList(lines []string) ([]User, string, bool) {
	bestStart, bestEnd, bestCount := -1, -1, 0
	for i := 0; i < len(lines); {
		if _, _, ok := structuredLogin(lines[i]); !ok {
			i++
			continue
		}
		start := i
		seen := map[string]bool{}
		for i < len(lines) {
			login, _, ok := structuredLogin(lines[i])
			if !ok {
				break
			}
			seen[login] = true
			i++
		}
		if len(seen) > bestCount {
			bestCount, bestStart, bestEnd = len(seen), start, i
		}
	}
	if bestCount < 2 {
		return nil, "", false
	}

	var users []User
	seen := map[string]bool{}
	for _, ln := range lines[bestStart:bestEnd] {
		login, name, ok := structuredLogin(ln)
		if !ok || seen[login] {
			continue
		}
		seen[login] = true
		users = append(users, User{Login: login, Name: name})
	}
	return users, trimPreamble(lines[:bestStart]), true
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestParseGeneric|TestGeneric' -count=1 -v`
Expected: PASS (all five).

- [ ] **Step 7: Run the full existing suite to confirm no regression**

Run: `go test ./tui/ -count=1`
Expected: PASS. In particular the existing decline tests (`TestDeclineDaemonHelpDebian`, `TestDeclineBannerTildeTown`, `TestDeclineTelehackHeaderWithoutRows`, `TestDeclineInlineCueTypedHoleWithoutAvailableFingers`, etc.) must still pass — none of their bodies contains ≥2 structured-login lines.

- [ ] **Step 8: Commit**

```bash
gofmt -w tui/userlist.go tui/userlist_test.go
git add tui/userlist.go tui/userlist_test.go
git commit -m "feat(tui): generic structured-login list fallback"
```

---

## Task 2: Additive strong-context drill targets

**Files:**
- Modify: `tui/userlist.go`
- Test: `tui/userlist_test.go`

- [ ] **Step 1: Write failing tests for additive harvesting**

Add to `tui/userlist_test.go`:

```go
// --- Generic fallback: additive strong-context targets ---

func TestGenericHarvestsFingerCommandTarget(t *testing.T) {
	// A bare-login block opens the list; a "finger user@host" mention elsewhere
	// in the body is appended as a cross-host drill target.
	body := []byte("betsy\nMelchizedek\n\nContact me: finger bob@example.org\n")
	users, ok := ParseUsers(body)
	if !ok {
		t.Fatal("ParseUsers ok = false, want true")
	}
	if got, want := logins(users), []string{"betsy", "Melchizedek", "bob"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logins = %v, want %v (harvested target appended last)", got, want)
	}
	if users[2].Target != "bob@example.org" {
		t.Fatalf("users[2].Target = %q, want bob@example.org", users[2].Target)
	}
}

func TestGenericTargetsDoNotOpenAlone(t *testing.T) {
	// No structured-login block: a lone "finger user@host" mention in prose must
	// NOT open a list (additive-only rule). This is the graph.no shape.
	body := []byte("Weather via finger.\nUsage:\n    finger oslo@graph.no\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (targets are additive-only)")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestGenericHarvests|TestGenericTargetsDoNotOpenAlone' -count=1 -v`
Expected: FAIL — `TestGenericHarvestsFingerCommandTarget` fails because `bob` is not appended yet (`parseGenericList` returns only the two structured logins). `TestGenericTargetsDoNotOpenAlone` should already PASS (no structured logins ⇒ decline); confirm it does.

- [ ] **Step 3: Implement `appendHarvestedTargets` and call it**

In `tui/userlist.go`, add the function:

```go
// appendHarvestedTargets adds cross-host drill targets found anywhere in the
// body via the existing strong-signal regexes (finger:// URLs and
// "finger user@host" commands) — the same contexts parseFingerRing and
// parseSavaTable already trust. Targets are additive: this is called only after
// the structured-login gate has already opened the list, so a stray mention
// can never open a list on its own. Bare emails and @handles are not harvested.
// Server-supplied targets are pinned to port 79 later, at drill time, by the
// existing pinFingerPort path in app.go.
func appendHarvestedTargets(users []User, lines []string) []User {
	seen := map[string]bool{}
	for _, u := range users {
		if u.Target != "" {
			seen[u.Target] = true
		}
	}
	body := strings.Join(lines, "\n")
	for _, m := range fingerURLRe.FindAllStringSubmatch(body, -1) {
		host, login := m[1], m[2]
		target := login + "@" + host
		if seen[target] {
			continue
		}
		seen[target] = true
		users = append(users, User{Login: login, Name: host, Target: target})
	}
	for _, m := range fingerCommandRe.FindAllStringSubmatch(body, -1) {
		target := m[1] // already in login@host form
		if seen[target] {
			continue
		}
		seen[target] = true
		login := target
		if at := strings.IndexByte(target, '@'); at >= 0 {
			login = target[:at]
		}
		users = append(users, User{Login: login, Target: target})
	}
	return users
}
```

In `parseGenericList`, replace the final return so harvested targets are appended:

```go
	users = appendHarvestedTargets(users, lines)
	return users, trimPreamble(lines[:bestStart]), true
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestGenericHarvests|TestGenericTargetsDoNotOpenAlone' -count=1 -v`
Expected: PASS (both).

- [ ] **Step 5: Run the full suite**

Run: `go test ./tui/ -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w tui/userlist.go tui/userlist_test.go
git add tui/userlist.go tui/userlist_test.go
git commit -m "feat(tui): harvest additive finger drill targets in generic fallback"
```

---

## Task 3: Flag the generic list (`(best guess)` + preamble note)

**Files:**
- Modify: `tui/list.go`
- Modify: `tui/app.go:197` (call-site arg) and `tui/app_test.go:175` (call-site arg)
- Test: `tui/list_test.go`

- [ ] **Step 1: Write failing tests for the flag and note**

Add to `tui/list_test.go`:

```go
func TestGenericListTitleFlaggedBestGuess(t *testing.T) {
	users := []User{{Login: "betsy"}, {Login: "oleander"}}
	body := []byte("betsy\noleander\n")
	m := newListWithPreamble(testCommon(), hostTarget(t, "@unknown.host"), users, body, false, true)
	if !strings.Contains(m.list.Title, "(best guess)") {
		t.Fatalf("title = %q, want it to contain (best guess)", m.list.Title)
	}
	if !m.generic {
		t.Fatal("listModel.generic = false, want true")
	}
}

func TestGenericListPreambleHasViewRawNote(t *testing.T) {
	users := []User{{Login: "betsy"}, {Login: "oleander"}}
	body := []byte("betsy\noleander\n")
	m := newListWithPreamble(testCommon(), hostTarget(t, "@unknown.host"), users, body, false, true)
	if !strings.Contains(m.preamble, "press r") {
		t.Fatalf("preamble = %q, want it to mention the view-raw key", m.preamble)
	}
}

func TestRecognizedListNotFlagged(t *testing.T) {
	users := []User{{Login: "alrs"}, {Login: "dtracker"}}
	body := []byte(hostListBody())
	m := newListWithPreamble(testCommon(), hostTarget(t, "@tilde.team"), users, body, false, false)
	if strings.Contains(m.list.Title, "(best guess)") {
		t.Fatalf("title = %q, want no (best guess) for a recognized list", m.list.Title)
	}
	if m.generic {
		t.Fatal("listModel.generic = true, want false for a recognized list")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestGenericList|TestRecognizedListNotFlagged' -count=1 -v`
Expected: FAIL to compile — `newListWithPreamble` takes 5 args, not 6, and `listModel` has no `generic` field.

- [ ] **Step 3: Add the `generic` field to `listModel`**

In `tui/list.go`, extend the struct:

```go
type listModel struct {
	common   *commonModel
	list     list.Model
	host     finger.Target
	preamble string
	generic  bool
}
```

- [ ] **Step 4: Extend `newListWithPreamble` with the `generic` param**

Replace the function in `tui/list.go`:

```go
func newListWithPreamble(common *commonModel, host finger.Target, users []User, body []byte, incomplete, generic bool) listModel {
	m := newList(common, host, users)
	m.generic = generic
	if generic {
		// Heuristic list: tell the user this was a best-effort parse, not a
		// recognized format. Composes with the "(incomplete)" flag below.
		m.list.Title += " (best guess)"
	}
	if incomplete {
		// The response errored or was truncated; flag it so a partial list is
		// not presented as if it were complete.
		m.list.Title += " (incomplete)"
	}
	if parsed, ok := parseUserList(body); ok {
		m.preamble = parsed.preamble
	} else {
		m.preamble = extractListPreamble(body)
	}
	if generic {
		note := "Auto-detected from an unrecognized response — press r to view the raw text."
		if m.preamble != "" {
			m.preamble = note + "\n\n" + m.preamble
		} else {
			m.preamble = note
		}
	}
	m.setSize(common.width, common.height)
	return m
}
```

- [ ] **Step 5: Update both existing call sites to pass `generic`**

In `tui/app.go:197`, change the call to pass `parsed.generic` (Task 4 finalizes `routeFetch`; for now pass the value so the package compiles):

```go
			m.list = newListWithPreamble(m.common, entry.Target, parsed.users, entry.Body, incomplete, parsed.generic)
```

In `tui/app_test.go:175`, the existing test builds a non-generic menu list; pass `false`:

```go
	m.list = newListWithPreamble(m.common, host, users, body, false, false)
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestGenericList|TestRecognizedListNotFlagged' -count=1 -v`
Expected: PASS (all three).

- [ ] **Step 7: Run the full suite**

Run: `go test ./tui/ -count=1`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
gofmt -w tui/list.go tui/app.go tui/list_test.go tui/app_test.go
git add tui/list.go tui/app.go tui/list_test.go tui/app_test.go
git commit -m "feat(tui): flag generic list as (best guess) with view-raw note"
```

---

## Task 4: `routeFetch` wiring + `r` view-raw escape hatch

**Files:**
- Modify: `tui/app.go` (`routeFetch` comment; `handleKey` stateList branch)
- Test: `tui/app_test.go`

- [ ] **Step 1: Write failing model tests**

Add to `tui/app_test.go`:

```go
func genericListBody() string {
	// No Login header / online cue / "> " marker: forces the generic fallback.
	return "the crew:\nbetsy\nMelchizedek\nOleander\nStarbloom\n"
}

func TestGenericHostFetchOpensFlaggedList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@unknown.host")
	entry := Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}

	next, _ := m.Update(fetchResultMsg{entry: entry})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList", got.state)
	}
	if !got.list.generic {
		t.Fatal("list.generic = false, want true")
	}
	if !strings.Contains(got.list.list.Title, "(best guess)") {
		t.Fatalf("title = %q, want (best guess)", got.list.list.Title)
	}
}

func TestRViewsRawBodyOnGenericList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@unknown.host")
	entry := Entry{Target: target, Body: []byte(genericListBody()), Meta: finger.Meta{Addr: target.HostPort}}
	opened, _ := m.Update(fetchResultMsg{entry: entry})
	m = opened.(appModel)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'r'})
	got := next.(appModel)

	if got.state != stateReader {
		t.Fatalf("state = %d, want stateReader after r", got.state)
	}
	if !got.fromList {
		t.Fatal("fromList = false, want true after viewing raw")
	}
	if !strings.Contains(got.reader.viewport.View(), "Melchizedek") {
		t.Fatalf("reader viewport missing raw body: %q", got.reader.viewport.View())
	}

	// Esc returns to the generic list.
	back, _ := got.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if back.(appModel).state != stateList {
		t.Fatalf("state = %d, want stateList after Esc", back.(appModel).state)
	}
}

func TestRInertOnRecognizedList(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	target := hostTarget(t, "@tilde.team")
	entry := Entry{Target: target, Body: []byte(hostListBody()), Meta: finger.Meta{Addr: target.HostPort}}
	opened, _ := m.Update(fetchResultMsg{entry: entry})
	m = opened.(appModel)

	next, _ := m.Update(tea.KeyPressMsg{Code: 'r'})
	got := next.(appModel)

	if got.state != stateList {
		t.Fatalf("state = %d, want stateList (r must be inert on a recognized list)", got.state)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./tui/ -run 'TestGenericHostFetchOpensFlaggedList|TestRViewsRawBodyOnGenericList|TestRInertOnRecognizedList' -count=1 -v`
Expected: `TestGenericHostFetchOpensFlaggedList` PASSES already (Task 3 wired `routeFetch`); `TestRViewsRawBodyOnGenericList` FAILS (state stays `stateList` — `r` not bound); `TestRInertOnRecognizedList` PASSES already. Confirm the one failure is the `r`-binding test.

- [ ] **Step 3: Bind `r` to view raw on a generic list**

In `tui/app.go`, inside `handleKey`, the `stateList` branch's `switch key.Code`, add a case alongside `tea.KeyEsc` / `tea.KeyEnter`:

```go
		case 'r':
			// On a generic ("best guess") list only, show the cached raw host
			// body so the user can read the actual response when the heuristic
			// parse looks wrong. fromList=true so Esc returns to the list.
			if m.list.generic && m.hostList != nil {
				m.reader.setEntry(*m.hostList)
				m.state = stateReader
				m.fromList = true
				return true, m, nil
			}
```

(When the list is not generic, this case does not match the guard and falls through to `return false, m, nil` at the end of `handleKey`, so the list keeps default behavior.)

- [ ] **Step 4: Update the `routeFetch` doc comment**

In `tui/app.go`, extend the comment above `routeFetch` to note the generic case:

```go
// routeFetch is the single decision point for a completed fetch: a host
// response that parses opens the list (flagged "(best guess)" when only the
// generic fallback recognized it); everything else renders in the reader.
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./tui/ -run 'TestGenericHostFetchOpensFlaggedList|TestRViewsRawBodyOnGenericList|TestRInertOnRecognizedList' -count=1 -v`
Expected: PASS (all three).

- [ ] **Step 6: Commit**

```bash
gofmt -w tui/app.go tui/app_test.go
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): bind r to view raw body on generic (best guess) list"
```

---

## Task 5: Real-capture decline corpus + full CI gates

**Files:**
- Modify: `tui/userlist_test.go`

- [ ] **Step 1: Write decline tests from real 2026-05-29 captures**

Add to `tui/userlist_test.go` (in the decline section). These are verbatim-shaped captures that must render in the reader, guarding the colon/single-space exclusion and the additive-only rule against regressions:

```go
func TestDeclineGraphNoWeatherHelp(t *testing.T) {
	// graph.no bare-host help: prose + "finger oslo@graph.no" usage example.
	// Must decline (oslo is a placeholder, not a user); additive-only rule.
	body := []byte("Weather via finger, graph.no\n\n" +
		"* Contact: finger@falkp.no\n\n" +
		"Usage:\n    finger oslo@graph.no\n\n" +
		"Using imperial units:\n    finger ^oslo@graph.no\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (graph.no usage help)")
	}
}

func TestDeclineDebianAttributeLegendFull(t *testing.T) {
	// Full db.debian.org attribute legend (10 "key : value" lines). Guards the
	// colon-form exclusion in the generic fallback.
	body := []byte("userdir-ldap finger daemon\n--------------------------\n" +
		"finger <uid>[/<attributes>]@db.debian.org\n" +
		"    The following attributes are currently supported:\n" +
		"      cn : First name\n      mn : Middle name\n      sn : Last name\n" +
		"      email : Email\n      labeleduri : URL\n      ircnick : IRC nickname\n" +
		"      icquin : ICQ UIN\n      jabberjid : Jabber ID\n" +
		"      keyfingerprint : Fingerprint\n      key : Key block\n")
	if _, ok := ParseUsers(body); ok {
		t.Fatal("ParseUsers ok = true, want false (LDAP attribute legend)")
	}
}
```

- [ ] **Step 2: Run the new decline tests**

Run: `go test ./tui/ -run 'TestDeclineGraphNoWeatherHelp|TestDeclineDebianAttributeLegendFull' -count=1 -v`
Expected: PASS (both decline).

- [ ] **Step 3: Run the three CI gates locally**

Run:
```bash
go vet ./...
gofmt -l .
go test ./... -race
```
Expected: `go vet` clean (no output); `gofmt -l .` prints nothing; `go test ./... -race` PASS. If `gofmt -l .` lists files, run `gofmt -w .` and re-check.

- [ ] **Step 4: Commit**

```bash
git add tui/userlist_test.go
git commit -m "test(tui): graph.no + debian legend decline corpus for generic fallback"
```

---

## Self-review notes (for the implementer)

- **Spec coverage:** structured-login gate (Task 1), additive strong-context targets (Task 2), `(best guess)` flag + preamble note (Task 3), `routeFetch` wiring + `r` escape hatch (Task 4), decline corpus incl. graph.no & db.debian.org (Tasks 1 & 5). Preemption guard is covered by running the full existing suite at the end of Tasks 1–3 (every list-bearing host stays claimed by its earlier parser).
- **Type consistency:** `parsedUserList.generic` (Task 1) → `newListWithPreamble(..., incomplete, generic bool)` (Task 3) → `listModel.generic` read by `handleKey` (Task 4). The view-raw key is the lowercase rune `'r'` (`key.Code == 'r'`), matching the existing `key.Code == 'c'` Ctrl+C idiom. `appendHarvestedTargets` returns the updated slice (value semantics), consistent with how `parseGenericList` reassigns `users`.
- **`pinFingerPort`:** harvested targets carry `User.Target` and are pinned to `:79` by the *existing* `drill()` path (`serverSupplied` ⇒ `pinFingerPort`); no new pinning code is needed. The parser tests assert the raw `Target` string only.
