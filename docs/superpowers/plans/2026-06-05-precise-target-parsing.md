# Precise Target Parsing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `finger.ParseTarget` validate forwarding, host/port, and bracketed IPv6 targets precisely before dialing.

**Architecture:** Keep `finger.ParseTarget` as the single target-validation chokepoint. Add an unexported `parseHostPort` helper in `finger/query.go` that returns canonical dial addresses, and add focused tests in `finger/query_test.go` plus a TUI pinning regression in `tui/app_test.go`.

**Tech Stack:** Go 1.26 toolchain, stdlib `net`, `strconv`, `strings`, existing `go test` and `make check` gates.

---

## File Structure

- Modify `finger/query.go`: import `net` and `strconv`, reject forwarding in `ParseTarget`, replace loose colon defaulting with `parseHostPort`, and add `parsePort`.
- Modify `finger/query_test.go`: extend the existing `TestParseTarget` table with forwarding, invalid port, and IPv6 cases.
- Modify `tui/app_test.go`: add one regression test near the existing `pinFingerPort` tests for bracketed IPv6 pinning.

No new package, dependency, or public API is needed.

---

### Task 1: Add Failing Finger Parser Tests

**Files:**
- Modify: `finger/query_test.go`
- Test: `finger/query_test.go`

- [ ] **Step 1: Extend the accepted target table**

In `finger/query_test.go`, add these cases to the success portion of `TestParseTarget`, after the existing scheme/path cases:

```go
{
	name:  "user with bracketed IPv6 defaults port",
	input: "alice@[::1]",
	want:  Target{User: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
},
{
	name:  "user with bracketed IPv6 explicit port",
	input: "alice@[::1]:7979",
	want:  Target{User: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
},
{
	name:  "host query with bracketed IPv6 defaults port",
	input: "@[::1]",
	want:  Target{User: "", HostPort: "[::1]:79", Raw: "@[::1]"},
},
{
	name:  "host query with bracketed IPv6 explicit port",
	input: "@[::1]:7979",
	want:  Target{User: "", HostPort: "[::1]:7979", Raw: "@[::1]:7979"},
},
{
	name:  "finger scheme with bracketed IPv6 path",
	input: "finger://[::1]/alice",
	want:  Target{User: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
},
{
	name:  "path-style bracketed IPv6 defaults port",
	input: "[::1]/alice",
	want:  Target{User: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
},
{
	name:  "path-style bracketed IPv6 explicit port",
	input: "[::1]:7979/alice",
	want:  Target{User: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
},
```

- [ ] **Step 2: Extend the rejected target table**

In the same `TestParseTarget` table, add these error cases near the existing error rows:

```go
{
	name:    "forwarded user query rejected for now",
	input:   "alice@plan.cat@tilde.team",
	wantErr: true,
},
{
	name:    "forwarded host query rejected for now",
	input:   "@plan.cat@tilde.team",
	wantErr: true,
},
{
	name:    "empty port",
	input:   "alice@example.com:",
	wantErr: true,
},
{
	name:    "non-numeric port",
	input:   "alice@example.com:abc",
	wantErr: true,
},
{
	name:    "out-of-range port",
	input:   "alice@example.com:99999",
	wantErr: true,
},
{
	name:    "zero port",
	input:   "alice@example.com:0",
	wantErr: true,
},
{
	name:    "unbracketed IPv6",
	input:   "alice@::1",
	wantErr: true,
},
{
	name:    "unclosed IPv6 bracket",
	input:   "alice@[::1",
	wantErr: true,
},
{
	name:    "bracketed IPv6 empty port",
	input:   "alice@[::1]:",
	wantErr: true,
},
{
	name:    "bracketed IPv6 non-numeric port",
	input:   "alice@[::1]:abc",
	wantErr: true,
},
```

- [ ] **Step 3: Run the focused parser test and confirm failures**

Run:

```bash
go test ./finger/ -run TestParseTarget -count=1 -v
```

Expected: the new bracketed IPv6 defaults and invalid port/forwarding cases fail under the current implementation.

---

### Task 2: Implement Precise Host/Port Parsing

**Files:**
- Modify: `finger/query.go`
- Test: `finger/query_test.go`

- [ ] **Step 1: Update imports**

Change the import block in `finger/query.go` to:

```go
import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)
```

- [ ] **Step 2: Replace the host/port block in `ParseTarget`**

Replace this existing block:

```go
user := arg[:at]
hostport := arg[at+1:]
if hostport == "" {
	return Target{}, errors.New("missing host after @")
}
if !strings.Contains(hostport, ":") {
	hostport = hostport + ":79"
}
if hasControl(user) || hasControl(hostport) {
	return Target{}, errors.New("target contains control characters")
}
return Target{User: user, HostPort: hostport, Raw: arg}, nil
```

with:

```go
user := arg[:at]
hostport := arg[at+1:]
if hostport == "" {
	return Target{}, errors.New("missing host after @")
}
if strings.Contains(hostport, "@") {
	return Target{}, errors.New("forwarded finger queries are not supported yet")
}
if hasControl(user) || hasControl(hostport) {
	return Target{}, errors.New("target contains control characters")
}
hostport, err := parseHostPort(hostport)
if err != nil {
	return Target{}, err
}
return Target{User: user, HostPort: hostport, Raw: arg}, nil
```

- [ ] **Step 3: Add `parseHostPort` and `parsePort` helpers**

Add these helpers below `hasControl` in `finger/query.go`:

```go
func parseHostPort(s string) (string, error) {
	if strings.HasPrefix(s, "[") {
		close := strings.IndexByte(s, ']')
		if close < 0 {
			return "", errors.New("IPv6 literals must be bracketed, e.g. [::1]")
		}
		host := s[1:close]
		if host == "" {
			return "", errors.New("missing host after @")
		}
		suffix := s[close+1:]
		if suffix == "" {
			return net.JoinHostPort(host, "79"), nil
		}
		if !strings.HasPrefix(suffix, ":") {
			return "", fmt.Errorf("invalid host/port %q", s)
		}
		port, err := parsePort(suffix[1:])
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(host, port), nil
	}

	switch strings.Count(s, ":") {
	case 0:
		if s == "" {
			return "", errors.New("missing host after @")
		}
		return net.JoinHostPort(s, "79"), nil
	case 1:
		host, port, _ := strings.Cut(s, ":")
		if host == "" {
			return "", errors.New("missing host after @")
		}
		port, err := parsePort(port)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(host, port), nil
	default:
		return "", errors.New("IPv6 literals must be bracketed, e.g. [::1]")
	}
}

func parsePort(s string) (string, error) {
	if s == "" {
		return "", errors.New("invalid port")
	}
	port, err := strconv.ParseUint(s, 10, 16)
	if err != nil || port == 0 {
		return "", errors.New("invalid port")
	}
	return strconv.FormatUint(port, 10), nil
}
```

- [ ] **Step 4: Run the focused parser test and confirm it passes**

Run:

```bash
go test ./finger/ -run TestParseTarget -count=1 -v
```

Expected: all `TestParseTarget` subtests pass.

---

### Task 3: Add IPv6 Pinning Regression

**Files:**
- Modify: `tui/app_test.go`
- Test: `tui/app_test.go`

- [ ] **Step 1: Add the pinning test**

In `tui/app_test.go`, add this test after `TestDrillServerSuppliedTargetPinnedToPort79`:

```go
func TestPinFingerPortKeepsBracketedIPv6(t *testing.T) {
	got := pinFingerPort(finger.Target{
		User:     "alice",
		HostPort: "[::1]:2222",
		Raw:      "alice@[::1]:2222",
	})

	if got.HostPort != "[::1]:79" {
		t.Fatalf("HostPort = %q, want [::1]:79", got.HostPort)
	}
	if got.Raw != "alice@[::1]:79" {
		t.Fatalf("Raw = %q, want alice@[::1]:79", got.Raw)
	}
}
```

- [ ] **Step 2: Run the focused TUI test**

Run:

```bash
go test ./tui/ -run TestPinFingerPortKeepsBracketedIPv6 -count=1 -v
```

Expected: pass. `pinFingerPort` already uses `net.SplitHostPort` and `net.JoinHostPort`, so this should be a regression guard rather than a code change.

---

### Task 4: Run Package and Full Gates

**Files:**
- Verify: all modified files

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./finger/ ./tui/ -count=1
```

Expected: both packages pass.

- [ ] **Step 2: Run formatting**

Run:

```bash
make fmt
```

Expected: no errors. If files change, inspect the diff before proceeding.

- [ ] **Step 3: Run full CI gate**

Run:

```bash
make check
```

Expected: `go vet ./...`, `golangci-lint run ./...`, and `go test ./... -race` all pass.

---

### Task 5: Review Diff and Commit

**Files:**
- Review: `finger/query.go`
- Review: `finger/query_test.go`
- Review: `tui/app_test.go`
- Review: `docs/superpowers/plans/2026-06-05-precise-target-parsing.md`

- [ ] **Step 1: Inspect the diff**

Run:

```bash
git diff -- finger/query.go finger/query_test.go tui/app_test.go docs/superpowers/plans/2026-06-05-precise-target-parsing.md
```

Expected: diff is limited to precise target parsing, tests, and this plan.

- [ ] **Step 2: Commit if the user has approved committing implementation work**

Run only after commit approval:

```bash
git add finger/query.go finger/query_test.go tui/app_test.go docs/superpowers/plans/2026-06-05-precise-target-parsing.md
git commit -m "feat(finger): parse targets precisely"
```

Expected: a conventional commit with no co-author or AI trailers.
