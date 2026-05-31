# tilde.team Pronouns highlighting & reflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On the `tilde.team` host only, highlight the `Pronouns:` finger field and reflow its inline value into a label-on-its-own-line block matching `Plan:`/`Project:`.

**Architecture:** All logic stays in `render/` (a pure function that already receives the target). A small new file `render/tildeteam.go` holds the host gate (`isTildeTeam`), the host-specific extra highlight prefix (`extraFieldPrefixes`), and the `reflowPronouns` body transform. `highlightFields` gains an `extra []string` parameter; generic `fieldPrefixes` is unchanged. `Render` applies the reflow then the highlight, both gated on the host.

**Tech Stack:** Go; `render/` package (uses lipgloss v1 + colorprofile). Tests are table/substring-based, offline.

Reference spec: `docs/superpowers/specs/2026-05-31-tilde-pronouns-design.md`.

---

### Task 1: Host gate + `Pronouns:` highlight (tilde.team only)

**Files:**
- Create: `render/tildeteam.go`
- Modify: `render/fields.go` (signature of `highlightFields`)
- Modify: `render/render.go` (pass `extraFieldPrefixes(t)` into `highlightFields`)
- Test: `render/tildeteam_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `render/tildeteam_test.go`:

```go
package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

func TestIsTildeTeam(t *testing.T) {
	cases := []struct {
		hostport string
		want     bool
	}{
		{"tilde.team:79", true},
		{"TILDE.TEAM:79", true},
		{"tilde.team", true},
		{"plan.cat:79", false},
		{"nottilde.team:79", false},
	}
	for _, c := range cases {
		got := isTildeTeam(finger.Target{HostPort: c.hostport})
		if got != c.want {
			t.Errorf("isTildeTeam(%q) = %v, want %v", c.hostport, got, c.want)
		}
	}
}

func TestRender_PronounsHighlightedOnTildeOnly(t *testing.T) {
	body := []byte("Pronouns: he/him\n")
	theme := NewTheme(colorprofile.TrueColor)
	styledLabel := theme.Field.Render("Pronouns:")

	tilde := finger.Target{HostPort: "tilde.team:79", Raw: "@tilde.team"}
	gotTilde := Render(tilde, body, finger.Meta{Addr: "tilde.team:79"}, nil, colorprofile.TrueColor)
	if !strings.Contains(gotTilde, styledLabel) {
		t.Errorf("tilde.team render should style the Pronouns label.\n--- got ---\n%s", gotTilde)
	}

	other := finger.Target{HostPort: "plan.cat:79", Raw: "@plan.cat"}
	gotOther := Render(other, body, finger.Meta{Addr: "plan.cat:79"}, nil, colorprofile.TrueColor)
	if strings.Contains(gotOther, styledLabel) {
		t.Errorf("non-tilde render must NOT style the Pronouns label.\n--- got ---\n%s", gotOther)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./render/ -run 'TestIsTildeTeam|TestRender_PronounsHighlightedOnTildeOnly' -v`
Expected: FAIL to compile — `undefined: isTildeTeam` (and later `extraFieldPrefixes`).

- [ ] **Step 3: Create `render/tildeteam.go` with the host gate**

```go
package render

import (
	"net"
	"strings"

	"github.com/jonathandeamer/lookit/finger"
)

// tildeTeamHost is the host whose finger daemon emits the server-specific
// "Pronouns:" field. Its Pronouns handling is gated to this host because
// Pronouns is a tilde.team convention, not a finger standard.
const tildeTeamHost = "tilde.team"

// isTildeTeam reports whether the target points at tilde.team (any port).
func isTildeTeam(t finger.Target) bool {
	host := t.HostPort
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.EqualFold(host, tildeTeamHost)
}

// extraFieldPrefixes returns host-specific label prefixes to highlight in
// addition to the generic finger fields. Only tilde.team contributes one
// ("Pronouns:"); every other host returns nil.
func extraFieldPrefixes(t finger.Target) []string {
	if isTildeTeam(t) {
		return []string{"Pronouns:"}
	}
	return nil
}
```

- [ ] **Step 4: Add the `extra` parameter to `highlightFields` in `render/fields.go`**

Replace the signature and the prefix loop so it also tries `extra`. The full updated function:

```go
// highlightFields walks body line by line and re-emits each line. If a line
// begins with one of fieldPrefixes (or extra), the prefix is wrapped in
// theme.Field; the rest of the line is untouched.
func highlightFields(theme Theme, body []byte, extra []string) string {
	if theme.NoColor {
		return string(body)
	}
	lines := strings.SplitAfter(string(body), "\n")
	var sb strings.Builder
	for _, line := range lines {
		matched := false
		for _, prefix := range fieldPrefixes {
			if strings.HasPrefix(line, prefix) {
				sb.WriteString(theme.Field.Render(prefix))
				sb.WriteString(line[len(prefix):])
				matched = true
				break
			}
		}
		if !matched {
			for _, prefix := range extra {
				if strings.HasPrefix(line, prefix) {
					sb.WriteString(theme.Field.Render(prefix))
					sb.WriteString(line[len(prefix):])
					matched = true
					break
				}
			}
		}
		if !matched {
			sb.WriteString(line)
		}
	}
	return sb.String()
}
```

- [ ] **Step 5: Update the `Render` call site in `render/render.go`**

Change the single `highlightFields` call (currently `sb.WriteString(highlightFields(theme, body))`) to:

```go
		sb.WriteString(highlightFields(theme, body, extraFieldPrefixes(t)))
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./render/ -run 'TestIsTildeTeam|TestRender_PronounsHighlightedOnTildeOnly' -v`
Expected: PASS.

- [ ] **Step 7: Run the whole render package (golden files must still pass)**

Run: `go test ./render/`
Expected: PASS — `basic`/`ascii-art` goldens unaffected (they use `plan.cat`, which contributes no extra prefix).

- [ ] **Step 8: Commit**

```bash
git add render/tildeteam.go render/fields.go render/render.go render/tildeteam_test.go
git commit -m "feat(render): highlight Pronouns field on tilde.team only"
```

---

### Task 2: Reflow inline `Pronouns:` into a block (tilde.team only)

**Files:**
- Modify: `render/tildeteam.go` (add `reflowPronouns`)
- Modify: `render/render.go` (apply reflow before highlight, gated on host)
- Test: `render/tildeteam_test.go` (add cases)

- [ ] **Step 1: Write the failing tests**

Append to `render/tildeteam_test.go`:

```go
func TestReflowPronouns(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"inline value", "Pronouns: he/him\n", "Pronouns:\n  he/him\n"},
		{"value with spaces", "Pronouns: she/her, ask\n", "Pronouns:\n  she/her, ask\n"},
		{"bare label untouched", "Pronouns:\n", "Pronouns:\n"},
		{"no pronouns line", "Plan:\n  hi\n", "Plan:\n  hi\n"},
		{"surrounded by blocks", "Plan:\n  hi\n\nPronouns: they/them\n", "Plan:\n  hi\n\nPronouns:\n  they/them\n"},
	}
	for _, c := range cases {
		got := string(reflowPronouns([]byte(c.in)))
		if got != c.want {
			t.Errorf("%s: reflowPronouns(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestRender_PronounsReflowedOnTildeOnly(t *testing.T) {
	body := []byte("Pronouns: he/him\n")

	tilde := finger.Target{HostPort: "tilde.team:79", Raw: "@tilde.team"}
	gotTilde := Render(tilde, body, finger.Meta{Addr: "tilde.team:79"}, nil, colorprofile.NoTTY)
	if !strings.Contains(gotTilde, "Pronouns:\n  he/him") {
		t.Errorf("tilde.team render should reflow Pronouns into a block.\n--- got ---\n%s", gotTilde)
	}

	other := finger.Target{HostPort: "plan.cat:79", Raw: "@plan.cat"}
	gotOther := Render(other, body, finger.Meta{Addr: "plan.cat:79"}, nil, colorprofile.NoTTY)
	if !strings.Contains(gotOther, "Pronouns: he/him") || strings.Contains(gotOther, "Pronouns:\n  he/him") {
		t.Errorf("non-tilde render must leave the Pronouns line inline.\n--- got ---\n%s", gotOther)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./render/ -run 'TestReflowPronouns|TestRender_PronounsReflowedOnTildeOnly' -v`
Expected: FAIL to compile — `undefined: reflowPronouns`.

- [ ] **Step 3: Add `reflowPronouns` to `render/tildeteam.go`**

Add this function (and ensure `strings` is already imported — it is, from Task 1):

```go
// pronounsInlinePrefix is the inline form emitted by tilde.team's daemon:
// "Pronouns:" followed by a single space and the value.
const pronounsInlinePrefix = "Pronouns: "

// reflowPronouns rewrites an inline "Pronouns: <value>" line into a block that
// matches the Plan:/Project: layout — the label on its own line and the value
// on the next line, indented two spaces. A bare "Pronouns:" (no value) and all
// other lines pass through unchanged. The body's line structure is otherwise
// preserved.
func reflowPronouns(body []byte) []byte {
	lines := strings.SplitAfter(string(body), "\n")
	var sb strings.Builder
	for _, line := range lines {
		// Split the trailing newline (if any) off so we reason about content.
		content := line
		nl := ""
		if strings.HasSuffix(content, "\n") {
			content = content[:len(content)-1]
			nl = "\n"
		}
		if strings.HasPrefix(content, pronounsInlinePrefix) {
			value := content[len(pronounsInlinePrefix):]
			if value != "" {
				sb.WriteString("Pronouns:\n  ")
				sb.WriteString(value)
				sb.WriteString(nl)
				continue
			}
		}
		sb.WriteString(line)
	}
	return []byte(sb.String())
}
```

- [ ] **Step 4: Apply the reflow in `render/render.go`**

In `Render`, in the non-empty-body branch, immediately before the `highlightFields` call added in Task 1, insert the host-gated reflow so the block reads:

```go
		if isTildeTeam(t) {
			body = reflowPronouns(body)
		}
		sb.WriteString(highlightFields(theme, body, extraFieldPrefixes(t)))
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./render/ -run 'TestReflowPronouns|TestRender_PronounsReflowedOnTildeOnly' -v`
Expected: PASS.

- [ ] **Step 6: Run the whole render package**

Run: `go test ./render/`
Expected: PASS — all four new tests plus unchanged goldens.

- [ ] **Step 7: Commit**

```bash
git add render/tildeteam.go render/render.go render/tildeteam_test.go
git commit -m "feat(render): reflow tilde.team Pronouns into a Plan-style block"
```

---

### Task 3: Full gate + live sanity check

**Files:** none (verification only)

- [ ] **Step 1: Run the full CI gate**

Run: `make check`
Expected: PASS — `go vet`, `gofmt -l` empty, `golangci-lint`, `go test ./... -race` all green. If `gofmt` flags a file, run `make fmt` and amend the relevant commit.

- [ ] **Step 2: Live visual check against the real server**

Run: `make build && ./lookit jonathan@tilde.team`
Expected: the `Pronouns:` label is colored like `Project:`/`Plan:`, and the value appears on its own indented line:
```
Pronouns:
  he/him
```
(Requires network. If offline, skip and note it.)

- [ ] **Step 3: Confirm non-tilde is unaffected**

Run: `./lookit @plan.cat` (or any non-tilde host that returns a profile)
Expected: no Pronouns reflow/highlight behavior introduced for other hosts; output unchanged from before this work. (Requires network. If offline, skip and note it.)

- [ ] **Step 4: No commit**

Verification only. If `make fmt` changed anything in Step 1, it was amended there.
