# Input forms & placeholder rotation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Accept `finger://` scheme and path-style (`host/user`) addresses everywhere, and rotate the TUI's greyed-out sample input over five real targets.

**Architecture:** All address-form normalization lands in `finger.ParseTarget` (a pre-pass that rewrites new forms into the existing `user@host` / `@host` forms before the current parser runs), so both the CLI and TUI benefit from one change. The placeholder rotation is a small TUI-only helper in `tui/app.go`.

**Tech Stack:** Go; `finger/` (no UI deps), `tui/` (Bubble Tea v2). Tests are table-driven, offline.

Reference spec: `docs/superpowers/specs/2026-05-31-input-forms-design.md`.

---

### Task 1: `ParseTarget` accepts `finger://` scheme and path-style addresses

**Files:**
- Modify: `finger/query.go:24-41` (the `ParseTarget` function)
- Test: `finger/query_test.go:11-47` (extend the existing `cases` table)

- [ ] **Step 1: Write the failing tests**

Add these rows to the `cases` slice in `TestParseTarget` (`finger/query_test.go`), after the existing `@host:port` case:

```go
{
    name:  "finger:// scheme with path",
    input: "finger://via.sour.is/xuu",
    want:  Target{User: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
},
{
    name:  "path-style, no scheme",
    input: "via.sour.is/xuu",
    want:  Target{User: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
},
{
    name:  "path-style with explicit port",
    input: "via.sour.is:7979/xuu",
    want:  Target{User: "xuu", HostPort: "via.sour.is:7979", Raw: "xuu@via.sour.is:7979"},
},
{
    name:  "finger:// scheme, host only -> host query",
    input: "finger://plan.cat",
    want:  Target{User: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
},
{
    name:  "finger:// scheme with userinfo",
    input: "finger://user@host",
    want:  Target{User: "user", HostPort: "host:79", Raw: "user@host"},
},
{
    name:  "path-style, trailing slash, empty user -> host query",
    input: "plan.cat/",
    want:  Target{User: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
},
{
    name:  "mixed-case scheme is stripped",
    input: "FINGER://via.sour.is/xuu",
    want:  Target{User: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./finger/ -run TestParseTarget -v`
Expected: FAIL — e.g. the `via.sour.is/xuu` case errors with "target must be of the form user@host or @host" (no `@`), and the `finger://` cases produce wrong `Raw`/`HostPort`.

- [ ] **Step 3: Implement the normalization pre-pass**

Replace the body of `ParseTarget` in `finger/query.go` so the new `normalize` step runs before the existing `@`-split logic. The existing logic below the call is unchanged:

```go
// ParseTarget parses one of these forms:
//
//	user@host
//	@host
//	user@host:port
//	@host:port
//
// It also accepts a leading "finger://" scheme and path-style "host[:port]/user"
// addresses (e.g. "finger://via.sour.is/xuu" or "via.sour.is/xuu"), normalizing
// them into the forms above. Returns an error for empty input or input with no
// "@", no "/", and no scheme, or for an empty host.
func ParseTarget(arg string) (Target, error) {
	if arg == "" {
		return Target{}, errors.New("empty target")
	}
	arg = normalizeTarget(arg)
	at := strings.Index(arg, "@")
	if at < 0 {
		return Target{}, errors.New("target must be of the form user@host or @host")
	}
	user := arg[:at]
	hostport := arg[at+1:]
	if hostport == "" {
		return Target{}, errors.New("missing host after @")
	}
	if !strings.Contains(hostport, ":") {
		hostport = hostport + ":79"
	}
	return Target{User: user, HostPort: hostport, Raw: arg}, nil
}

// normalizeTarget rewrites scheme-prefixed and path-style addresses into the
// canonical "user@host" / "@host" forms ParseTarget understands. Plain
// "@"-forms pass through untouched. A finger user is a single token, so only
// the first "/" separates host from user.
func normalizeTarget(arg string) string {
	hadScheme := false
	if i := strings.Index(arg, "://"); i >= 0 && strings.EqualFold(arg[:i], "finger") {
		arg = arg[i+len("://"):]
		hadScheme = true
	}
	arg = strings.TrimRight(arg, "/")

	if strings.Contains(arg, "@") {
		return arg // already canonical (covers finger://user@host)
	}
	if slash := strings.Index(arg, "/"); slash >= 0 {
		host := arg[:slash]
		user := arg[slash+1:]
		return user + "@" + host // user may be "" -> "@host"
	}
	if hadScheme {
		return "@" + arg // finger://host with no path is a bare host query
	}
	return arg // bare token, no @/scheme/slash: let ParseTarget reject it
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./finger/ -run TestParseTarget -v`
Expected: PASS for all rows, including the unchanged existing ones (their `Raw` still equals the original input).

- [ ] **Step 5: Commit**

```bash
git add finger/query.go finger/query_test.go
git commit -m "feat(finger): accept finger:// scheme and path-style targets"
```

---

### Task 2: Rotating sample placeholder in the TUI

**Files:**
- Modify: `tui/app.go:94` (the `in.Placeholder = "alice@plan.cat"` line in `newApp`) and add a package-level `sampleTargets` var + `pickSample` helper near it
- Test: `tui/app_test.go` (add a new test function)

- [ ] **Step 1: Write the failing test**

Add this test to `tui/app_test.go` (any location in the file):

```go
func TestPickSampleIsMember(t *testing.T) {
	for i := 0; i < 50; i++ {
		got := pickSample()
		found := false
		for _, s := range sampleTargets {
			if got == s {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("pickSample() = %q, not in sampleTargets", got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tui/ -run TestPickSampleIsMember -v`
Expected: FAIL to compile — `undefined: pickSample` and `undefined: sampleTargets`.

- [ ] **Step 3: Implement `sampleTargets` and `pickSample`**

In `tui/app.go`, add `"math/rand"` to the import block (alphabetically after `"math"`). Then add this above `newApp`:

```go
// sampleTargets are the rotating greyed-out hints shown in the empty target
// input. The mix of "@host" directory shapes and "user@host" profile shapes
// teaches both input forms. They are hint text only, never auto-submitted.
var sampleTargets = []string{
	"ring@thebackupbox.net",
	"@happynetbox.com",
	"@plan.cat",
	"@tilde.team",
	"jonathan@tilde.team",
}

// pickSample returns a uniformly random sample target for the placeholder.
func pickSample() string {
	return sampleTargets[rand.Intn(len(sampleTargets))]
}
```

Then change the placeholder assignment in `newApp`:

```go
	in.Placeholder = pickSample()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tui/ -run TestPickSampleIsMember -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tui/app.go tui/app_test.go
git commit -m "feat(tui): rotate the sample target placeholder per launch"
```

---

### Task 3: Full gate + manual sanity check

**Files:** none (verification only)

- [ ] **Step 1: Run the full CI gate**

Run: `make check`
Expected: PASS — `go vet`, `gofmt -l` empty, `golangci-lint`, and `go test ./... -race` all green. If `gofmt` flags a file, run `make fmt` and amend the relevant commit.

- [ ] **Step 2: Manual CLI smoke (offline parse path only)**

Build and confirm the new forms reach the network layer (a real fetch will fail offline; we only care that parsing no longer errors with a usage message):

Run: `make build && ./lookit finger://via.sour.is/xuu; echo "exit=$?"`
Expected: NOT a usage error (exit 64). It attempts a fetch and exits 0 or 2 (network) depending on connectivity. `./lookit nonsense` should still print the usage error and exit 64.

- [ ] **Step 3: No commit**

Verification only — nothing to commit. If `make fmt` changed anything in Step 1, it was amended there.
