# One-Relay Forwarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safe typed support for one RFC 1288 forwarding hop, such as `jonathan@tilde.team@thebackupbox.net`.

**Architecture:** Add an explicit `Target.Query` field for the exact wire query while keeping `Target.User` as a compatibility alias during migration. `ParseTarget` accepts exactly one typed relay, `ParseTargetPinned` rejects all forwarding for server-supplied targets, and `finger.Query` writes only `Target.QueryLine() + "\r\n"` to the socket.

**Tech Stack:** Go 1.26 toolchain, stdlib `net`/`strings`/`errors`, existing `finger` fake server tests, Bubble Tea v2 model tests, `make check`.

---

## File Structure

- Modify `finger/query.go`:
  - add `Target.Query`
  - add `Target.QueryLine()` and `Target.HostQuery()`
  - add forwarding-specific error values
  - parse direct targets and exactly-one-relay targets separately
  - keep `ParseTargetPinned` direct-only and port-pinned
- Modify `finger/client.go`:
  - write `Target.QueryLine()` to the server
  - reject controls in `Target.QueryLine()` before writing
- Modify `finger/query_test.go`:
  - add `Query` to successful expected targets
  - cover accepted one-relay forms and targeted error messages
  - prove `ParseTargetPinned` rejects server-supplied forwarding with the server-specific error
- Modify `finger/client_test.go`:
  - update manual `Target` literals where clarity requires `Query`
  - add wire tests proving forwarded inputs send only the left-side query to the dialed relay
  - update the control-character egress guard to use `Query`
- Modify `finger/fuzz_test.go`:
  - assert accepted parse targets have no controls in `QueryLine()` or `HostPort`
- Modify `tui/app.go`:
  - surface the server-supplied forwarding refusal as a status flash in drill/copy paths
  - route host/list forwarded queries via `Target.HostQuery()`
- Modify `tui/statusbar.go`:
  - use `Target.QueryLine()` in breadcrumbs so forwarded targets show the inner query
- Modify `tui/app_test.go`:
  - record full fetched targets for typed forwarding
  - add server-supplied forwarding refusal tests

Do not modify `render/` in this plan. It already renders `Target.Raw`, which remains the user-visible address.

---

### Task 1: Add the Explicit Query Model for Direct Targets

**Files:**
- Modify: `finger/query.go`
- Modify: `finger/client.go`
- Modify: `finger/query_test.go`
- Modify: `finger/client_test.go`
- Modify: `finger/fuzz_test.go`
- Test: `finger/query_test.go`, `finger/client_test.go`, `finger/fuzz_test.go`

- [ ] **Step 1: Add failing direct-target expectations**

In `finger/query_test.go`, update every successful `want: Target{...}` in `TestParseTarget` and `TestParseTargetPinned` so direct user queries include `Query` matching the user, and host/list queries include `Query: ""`.

Use this exact pattern for the existing cases:

```go
{
	name:  "user@host",
	input: "alice@plan.cat",
	want:  Target{User: "alice", Query: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"},
},
{
	name:  "@host (server query)",
	input: "@tilde.team",
	want:  Target{User: "", Query: "", HostPort: "tilde.team:79", Raw: "@tilde.team"},
},
{
	name:  "user@host:port",
	input: "alice@example.com:7979",
	want:  Target{User: "alice", Query: "alice", HostPort: "example.com:7979", Raw: "alice@example.com:7979"},
},
{
	name:  "@host:port",
	input: "@example.com:7979",
	want:  Target{User: "", Query: "", HostPort: "example.com:7979", Raw: "@example.com:7979"},
},
{
	name:  "finger:// scheme with path",
	input: "finger://via.sour.is/xuu",
	want:  Target{User: "xuu", Query: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
},
{
	name:  "path-style, no scheme",
	input: "via.sour.is/xuu",
	want:  Target{User: "xuu", Query: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
},
{
	name:  "path-style with explicit port",
	input: "via.sour.is:7979/xuu",
	want:  Target{User: "xuu", Query: "xuu", HostPort: "via.sour.is:7979", Raw: "xuu@via.sour.is:7979"},
},
{
	name:  "finger:// scheme, host only -> host query",
	input: "finger://plan.cat",
	want:  Target{User: "", Query: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
},
{
	name:  "finger:// scheme with userinfo",
	input: "finger://user@host",
	want:  Target{User: "user", Query: "user", HostPort: "host:79", Raw: "user@host"},
},
{
	name:  "path-style, trailing slash, empty user -> host query",
	input: "plan.cat/",
	want:  Target{User: "", Query: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
},
{
	name:  "mixed-case scheme is stripped",
	input: "FINGER://via.sour.is/xuu",
	want:  Target{User: "xuu", Query: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
},
{
	name:  "user with bracketed IPv6 defaults port",
	input: "alice@[::1]",
	want:  Target{User: "alice", Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
},
{
	name:  "user with bracketed IPv6 explicit port",
	input: "alice@[::1]:7979",
	want:  Target{User: "alice", Query: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
},
{
	name:  "host query with bracketed IPv6 defaults port",
	input: "@[::1]",
	want:  Target{User: "", Query: "", HostPort: "[::1]:79", Raw: "@[::1]"},
},
{
	name:  "host query with bracketed IPv6 explicit port",
	input: "@[::1]:7979",
	want:  Target{User: "", Query: "", HostPort: "[::1]:7979", Raw: "@[::1]:7979"},
},
{
	name:  "finger scheme with bracketed IPv6 path",
	input: "finger://[::1]/alice",
	want:  Target{User: "alice", Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
},
{
	name:  "path-style bracketed IPv6 defaults port",
	input: "[::1]/alice",
	want:  Target{User: "alice", Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
},
{
	name:  "path-style bracketed IPv6 explicit port",
	input: "[::1]:7979/alice",
	want:  Target{User: "alice", Query: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
},
```

For `TestParseTargetPinned`, use this exact expected shape:

```go
{
	name:  "hostile port pinned to 79 and surfaced in Raw",
	input: "evil@example.com:22",
	want:  Target{User: "evil", Query: "evil", HostPort: "example.com:79", Raw: "evil@example.com:79"},
},
{
	name:  "out-of-range port discarded, not rejected",
	input: "alice@example.com:99999",
	want:  Target{User: "alice", Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
},
{
	name:  "zero port discarded, not rejected",
	input: "alice@example.com:0",
	want:  Target{User: "alice", Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
},
{
	name:  "no explicit port keeps clean Raw",
	input: "yalla@tilde.team",
	want:  Target{User: "yalla", Query: "yalla", HostPort: "tilde.team:79", Raw: "yalla@tilde.team"},
},
{
	name:  "explicit :79 keeps clean Raw",
	input: "alice@example.com:79",
	want:  Target{User: "alice", Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
},
{
	name:  "bracketed IPv6 port pinned",
	input: "alice@[::1]:2222",
	want:  Target{User: "alice", Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]:79"},
},
{
	name:  "finger scheme link with hostile port pinned",
	input: "finger://example.com:31337/alice",
	want:  Target{User: "alice", Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
},
```

- [ ] **Step 2: Run focused parser tests and confirm the model field is missing**

Run:

```bash
go test ./finger/ -run TestParseTarget -count=1 -v
```

Expected: FAIL to compile with an error like `unknown field Query in struct literal of type Target`.

- [ ] **Step 3: Add `Query` and query helper methods**

In `finger/query.go`, replace the `Target` struct with:

```go
// Target identifies a finger query and the host:port endpoint that receives it.
type Target struct {
	User     string // deprecated compatibility alias for Query
	Query    string // exact Finger query line without trailing CRLF
	HostPort string // always "host:port"; port defaults to "79"
	Raw      string // normalized argument string, e.g. "alice@plan.cat"
}

// QueryLine returns the exact line Query should send, without the trailing CRLF.
// The User fallback keeps manually-constructed legacy test targets working while
// call sites migrate to Query.
func (t Target) QueryLine() string {
	if t.Query != "" || t.User == "" {
		return t.Query
	}
	return t.User
}

// HostQuery reports whether a response can be treated as a host/list response.
// RFC 1288 forwarding permits a query like "@host@relay"; after parsing, that
// has Query "@host" and should route like a host query if the body is parseable.
func (t Target) HostQuery() bool {
	q := t.QueryLine()
	return q == "" || strings.HasPrefix(q, "@")
}
```

- [ ] **Step 4: Populate `Query` for direct parsed targets**

In `finger/query.go`, change the return at the end of `parseTarget` from:

```go
return Target{User: user, HostPort: canonical, Raw: raw}, nil
```

to:

```go
return Target{User: user, Query: user, HostPort: canonical, Raw: raw}, nil
```

- [ ] **Step 5: Make `finger.Query` write `QueryLine()`**

In `finger/client.go`, replace the query write block:

```go
if hasControl(t.User) {
	meta.Elapsed = time.Since(start)
	return nil, meta, fmt.Errorf("query user contains control characters")
}
if _, err := fmt.Fprintf(conn, "%s\r\n", t.User); err != nil {
	meta.Elapsed = time.Since(start)
	return nil, meta, fmt.Errorf("write query: %w", err)
}
```

with:

```go
query := t.QueryLine()
if hasControl(query) {
	meta.Elapsed = time.Since(start)
	return nil, meta, fmt.Errorf("query contains control characters")
}
if _, err := fmt.Fprintf(conn, "%s\r\n", query); err != nil {
	meta.Elapsed = time.Since(start)
	return nil, meta, fmt.Errorf("write query: %w", err)
}
```

- [ ] **Step 6: Update the control-character query test**

In `finger/client_test.go`, rename `TestQueryRejectsControlCharsInUser` to `TestQueryRejectsControlCharsInQuery` and replace:

```go
tgt := Target{User: "a\r\nb", HostPort: ln.Addr().String()}
```

with:

```go
tgt := Target{Query: "a\r\nb", HostPort: ln.Addr().String()}
```

Update its failure text from:

```go
t.Fatal("Query with control char in user = nil error, want error")
```

to:

```go
t.Fatal("Query with control char in query = nil error, want error")
```

- [ ] **Step 7: Update parse fuzz security assertion**

In `finger/fuzz_test.go`, replace:

```go
if hasControl(target.User) || hasControl(target.HostPort) {
	t.Fatalf("ParseTarget accepted %q but target carries control bytes: %+v", arg, target)
}
```

with:

```go
if hasControl(target.QueryLine()) || hasControl(target.HostPort) {
	t.Fatalf("ParseTarget accepted %q but target carries control bytes: %+v", arg, target)
}
```

- [ ] **Step 8: Run focused direct-target tests**

Run:

```bash
go test ./finger/ -run 'TestParseTarget|TestQuery_UserHappyPath|TestQuery_ServerForm|TestQueryRejectsControlCharsInQuery' -count=1 -v
```

Expected: PASS. Direct behavior is unchanged, and `Query` is populated for parser-created targets.

- [ ] **Step 9: Commit this task if commit approval has been granted**

Run only if the user has explicitly approved commits:

```bash
git add finger/query.go finger/client.go finger/query_test.go finger/client_test.go finger/fuzz_test.go
git commit -m "refactor(finger): make wire query explicit"
```

Expected: a conventional commit with no co-author or AI trailers.

---

### Task 2: Add One-Relay Parser Support and Targeted Errors

**Files:**
- Modify: `finger/query.go`
- Modify: `finger/query_test.go`
- Test: `finger/query_test.go`

- [ ] **Step 1: Add forwarding parser tests**

In `finger/query_test.go`, add these success cases to `TestParseTarget` after the current direct success cases:

```go
{
	name:  "forwarded user query",
	input: "alice@tilde.team@thebackupbox.net",
	want:  Target{User: "alice@tilde.team", Query: "alice@tilde.team", HostPort: "thebackupbox.net:79", Raw: "alice@tilde.team@thebackupbox.net"},
},
{
	name:  "forwarded host query",
	input: "@tilde.team@thebackupbox.net",
	want:  Target{User: "@tilde.team", Query: "@tilde.team", HostPort: "thebackupbox.net:79", Raw: "@tilde.team@thebackupbox.net"},
},
{
	name:  "forwarded user query with relay port",
	input: "alice@tilde.team@thebackupbox.net:7979",
	want:  Target{User: "alice@tilde.team", Query: "alice@tilde.team", HostPort: "thebackupbox.net:7979", Raw: "alice@tilde.team@thebackupbox.net:7979"},
},
{
	name:  "forwarded host query with relay port",
	input: "@tilde.team@thebackupbox.net:7979",
	want:  Target{User: "@tilde.team", Query: "@tilde.team", HostPort: "thebackupbox.net:7979", Raw: "@tilde.team@thebackupbox.net:7979"},
},
{
	name:  "forwarded user query with inner bracketed IPv6 host",
	input: "alice@[::1]@thebackupbox.net",
	want:  Target{User: "alice@[::1]", Query: "alice@[::1]", HostPort: "thebackupbox.net:79", Raw: "alice@[::1]@thebackupbox.net"},
},
```

In the error rows of `TestParseTarget`, replace the old forwarded rejection cases with message-aware cases. First update the table type to include `wantErr string`:

```go
cases := []struct {
	name    string
	input   string
	want    Target
	wantErr string
}{
```

Then replace the error branch in the test loop with:

```go
if tc.wantErr != "" {
	if err == nil {
		t.Fatalf("ParseTarget(%q): expected error %q, got nil", tc.input, tc.wantErr)
	}
	if got := err.Error(); got != tc.wantErr {
		t.Fatalf("ParseTarget(%q) error = %q, want %q", tc.input, got, tc.wantErr)
	}
	return
}
```

Use these rows for the existing non-forwarding error cases:

```go
{
	name:    "missing @",
	input:   "alice",
	wantErr: "target must be of the form user@host or @host",
},
{
	name:    "empty",
	input:   "",
	wantErr: "empty target",
},
{
	name:    "@ with no host",
	input:   "alice@",
	wantErr: "missing host after @",
},
{
	name:    "empty port",
	input:   "alice@example.com:",
	wantErr: "invalid port",
},
{
	name:    "non-numeric port",
	input:   "alice@example.com:abc",
	wantErr: "invalid port",
},
{
	name:    "out-of-range port",
	input:   "alice@example.com:99999",
	wantErr: "invalid port",
},
{
	name:    "zero port",
	input:   "alice@example.com:0",
	wantErr: "invalid port",
},
{
	name:    "unbracketed IPv6",
	input:   "alice@::1",
	wantErr: "IPv6 literals must be bracketed, e.g. [::1]",
},
{
	name:    "unclosed IPv6 bracket",
	input:   "alice@[::1",
	wantErr: "IPv6 literals must be bracketed, e.g. [::1]",
},
{
	name:    "bracketed IPv6 empty port",
	input:   "alice@[::1]:",
	wantErr: "invalid port",
},
{
	name:    "bracketed IPv6 non-numeric port",
	input:   "alice@[::1]:abc",
	wantErr: "invalid port",
},
{
	name:    "control char CR+LF in user",
	input:   "a\r\nb@host",
	wantErr: "target contains control characters",
},
{
	name:    "control char NUL in host",
	input:   "u@ho\x00st",
	wantErr: "target contains control characters",
},
{
	name:    "DEL in user",
	input:   "a\x7f@host",
	wantErr: "target contains control characters",
},
```

Use this list for forwarding-specific rows:

```go
{
	name:    "multiple forwarding relays rejected",
	input:   "alice@h1@h2@relay",
	wantErr: "forwarding through multiple relays is not supported yet",
},
{
	name:    "multiple forwarding relays rejected for host query",
	input:   "@h1@h2@relay",
	wantErr: "forwarding through multiple relays is not supported yet",
},
{
	name:    "inner forwarded user port rejected",
	input:   "alice@tilde.team:7979@thebackupbox.net",
	wantErr: "forwarded host ports are not supported; put a port only on the relay",
},
{
	name:    "inner forwarded host port rejected",
	input:   "@tilde.team:7979@thebackupbox.net",
	wantErr: "forwarded host ports are not supported; put a port only on the relay",
},
{
	name:    "URL forwarding rejected with deferred message",
	input:   "finger://thebackupbox.net/alice@tilde.team",
	wantErr: "forwarding in finger:// URLs is not supported yet; use user@host@relay",
},
{
	name:    "malformed forwarding missing inner host",
	input:   "alice@@thebackupbox.net",
	wantErr: "forwarded targets must be user@host@relay or @host@relay",
},
```

- [ ] **Step 2: Make pinned parser errors message-aware**

In `finger/query_test.go`, update the `TestParseTargetPinned` table type to include `wantErr string`:

```go
cases := []struct {
	name    string
	input   string
	want    Target
	wantErr string
}{
```

Replace the error branch in the `TestParseTargetPinned` loop with:

```go
if tc.wantErr != "" {
	if err == nil {
		t.Fatalf("ParseTargetPinned(%q): expected error %q, got nil", tc.input, tc.wantErr)
	}
	if got := err.Error(); got != tc.wantErr {
		t.Fatalf("ParseTargetPinned(%q) error = %q, want %q", tc.input, got, tc.wantErr)
	}
	return
}
```

Use these pinned error cases:

```go
{
	name:    "unbracketed IPv6 still rejected",
	input:   "alice@fe80::1",
	wantErr: "IPv6 literals must be bracketed, e.g. [::1]",
},
{
	name:    "control char still rejected",
	input:   "a\r\nb@host",
	wantErr: "target contains control characters",
},
{
	name:    "forwarded query still rejected",
	input:   "alice@plan.cat@tilde.team",
	wantErr: ErrServerForwarding.Error(),
},
{
	name:    "forwarded host query still rejected",
	input:   "@plan.cat@tilde.team",
	wantErr: ErrServerForwarding.Error(),
},
{
	name:    "multiple forwarded relays still rejected",
	input:   "alice@h1@h2@relay",
	wantErr: ErrServerForwarding.Error(),
},
```

- [ ] **Step 3: Run parser tests and confirm forwarding still fails**

Run:

```bash
go test ./finger/ -run TestParseTarget -count=1 -v
```

Expected: FAIL. Accepted forwarding cases should still return an error from the current parser.

- [ ] **Step 4: Add forwarding error values**

In `finger/query.go`, extend the error var block to:

```go
var (
	errMissingHost           = errors.New("missing host after @")
	errBracketIPv6           = errors.New("IPv6 literals must be bracketed, e.g. [::1]")
	errMultipleRelays        = errors.New("forwarding through multiple relays is not supported yet")
	errForwardedHostPort     = errors.New("forwarded host ports are not supported; put a port only on the relay")
	errURLForwarding         = errors.New("forwarding in finger:// URLs is not supported yet; use user@host@relay")
	errMalformedForwarding   = errors.New("forwarded targets must be user@host@relay or @host@relay")
	ErrServerForwarding      = errors.New("forwarded targets from server responses are not opened")
)
```

Use `ErrServerForwarding` from `tui` in Task 4; it is exported because `tui` is a separate package.

- [ ] **Step 5: Replace `parseTarget` with direct/forwarding dispatch**

In `finger/query.go`, replace the current `parseTarget` body with:

```go
func parseTarget(arg string, pin bool) (Target, error) {
	if arg == "" {
		return Target{}, errors.New("empty target")
	}
	if isDeferredURLForwarding(arg) {
		return Target{}, errURLForwarding
	}
	arg = normalizeTarget(arg)
	if hasControl(arg) {
		return Target{}, errors.New("target contains control characters")
	}

	switch strings.Count(arg, "@") {
	case 0:
		return Target{}, errors.New("target must be of the form user@host or @host")
	case 1:
		return parseDirectTarget(arg, pin)
	case 2:
		if pin {
			return Target{}, ErrServerForwarding
		}
		return parseForwardedTarget(arg)
	default:
		if pin {
			return Target{}, ErrServerForwarding
		}
		return Target{}, errMultipleRelays
	}
}
```

- [ ] **Step 6: Add direct and forwarded target helpers**

In `finger/query.go`, add these helpers below `parseTarget`:

```go
func parseDirectTarget(arg string, pin bool) (Target, error) {
	user, hostport, _ := strings.Cut(arg, "@")
	if hostport == "" {
		return Target{}, errMissingHost
	}
	host, rawPort, hasPort, err := splitHostPort(hostport)
	if err != nil {
		return Target{}, err
	}

	port := "79"
	if !pin && hasPort {
		if port, err = parsePort(rawPort); err != nil {
			return Target{}, err
		}
	}
	canonical := net.JoinHostPort(host, port)
	raw := arg
	if pin && hasPort && rawPort != "79" {
		raw = user + "@" + canonical
	}
	return Target{User: user, Query: user, HostPort: canonical, Raw: raw}, nil
}

func parseForwardedTarget(arg string) (Target, error) {
	last := strings.LastIndex(arg, "@")
	if last <= 0 || last == len(arg)-1 {
		return Target{}, errMalformedForwarding
	}
	query := arg[:last]
	relay := arg[last+1:]

	if err := validateForwardQuery(query); err != nil {
		return Target{}, err
	}
	host, rawPort, hasPort, err := splitHostPort(relay)
	if err != nil {
		return Target{}, err
	}
	port := "79"
	if hasPort {
		if port, err = parsePort(rawPort); err != nil {
			return Target{}, err
		}
	}
	return Target{User: query, Query: query, HostPort: net.JoinHostPort(host, port), Raw: arg}, nil
}

func validateForwardQuery(query string) error {
	user, host, ok := strings.Cut(query, "@")
	if !ok || host == "" {
		return errMalformedForwarding
	}
	if user == "" && !strings.HasPrefix(query, "@") {
		return errMalformedForwarding
	}
	_, _, hasPort, err := splitHostPort(host)
	if err != nil {
		return err
	}
	if hasPort {
		return errForwardedHostPort
	}
	return nil
}
```

- [ ] **Step 7: Add deferred URL forwarding detection**

In `finger/query.go`, add this helper near `normalizeTarget`:

```go
func isDeferredURLForwarding(arg string) bool {
	i := strings.Index(arg, "://")
	if i < 0 || !strings.EqualFold(arg[:i], "finger") {
		return false
	}
	rest := arg[i+len("://"):]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return false
	}
	return strings.Contains(rest[slash+1:], "@")
}
```

- [ ] **Step 8: Run parser tests**

Run:

```bash
go test ./finger/ -run TestParseTarget -count=1 -v
```

Expected: PASS. The parser now splits forwarded targets at the rightmost `@`, validates the left side as `user@host` or `@host`, and parses the right side as the relay.

- [ ] **Step 9: Commit this task if commit approval has been granted**

Run only if the user has explicitly approved commits:

```bash
git add finger/query.go finger/query_test.go
git commit -m "feat(finger): parse one-relay forwarding targets"
```

Expected: a conventional commit with no co-author or AI trailers.

---

### Task 3: Prove Forwarded Wire Queries

**Files:**
- Modify: `finger/client_test.go`
- Test: `finger/client_test.go`

- [ ] **Step 1: Add wire tests for forwarded user and host queries**

In `finger/client_test.go`, add these tests after `TestQuery_ServerForm`:

```go
func TestQuery_ForwardedUserQueryWritesRemainder(t *testing.T) {
	fs := newFakeServer(t, func(line string) []byte {
		return []byte("forwarded profile\r\n")
	})

	target, err := ParseTarget("alice@tilde.team@" + fs.addr)
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, meta, err := Query(ctx, target)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if fs.gotLine != "alice@tilde.team\r\n" {
		t.Fatalf("server received %q, want %q", fs.gotLine, "alice@tilde.team\r\n")
	}
	if meta.Addr != fs.addr {
		t.Fatalf("Addr = %q, want %q", meta.Addr, fs.addr)
	}
	if string(body) != "forwarded profile\n" {
		t.Fatalf("body = %q, want forwarded profile", body)
	}
}

func TestQuery_ForwardedHostQueryWritesRemainder(t *testing.T) {
	fs := newFakeServer(t, func(line string) []byte {
		return []byte("forwarded list\r\n")
	})

	target, err := ParseTarget("@tilde.team@" + fs.addr)
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, _, err := Query(ctx, target)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if fs.gotLine != "@tilde.team\r\n" {
		t.Fatalf("server received %q, want %q", fs.gotLine, "@tilde.team\r\n")
	}
	if string(body) != "forwarded list\n" {
		t.Fatalf("body = %q, want forwarded list", body)
	}
}
```

- [ ] **Step 2: Run the forwarded wire tests**

Run:

```bash
go test ./finger/ -run 'TestQuery_Forwarded(User|Host)QueryWritesRemainder' -count=1 -v
```

Expected: PASS. These tests prove lookit dials the rightmost relay and sends only the RFC remainder.

- [ ] **Step 3: Run all finger tests**

Run:

```bash
go test ./finger/ -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit this task if commit approval has been granted**

Run only if the user has explicitly approved commits:

```bash
git add finger/client_test.go
git commit -m "test(finger): prove one-relay forwarding wire query"
```

Expected: a conventional commit with no co-author or AI trailers.

---

### Task 4: Wire Forwarding Semantics Through the TUI

**Files:**
- Modify: `tui/app.go`
- Modify: `tui/statusbar.go`
- Modify: `tui/app_test.go`
- Test: `tui/app_test.go`

- [ ] **Step 1: Add a target-recording helper for full targets**

In `tui/app_test.go`, add this helper near `fetchRecorder`:

```go
func fetchTargetRecorder(body string) (FetchFunc, *[]finger.Target) {
	var seen []finger.Target
	f := func(_ context.Context, t finger.Target) ([]byte, finger.Meta, error) {
		seen = append(seen, t)
		return []byte(body), finger.Meta{Addr: t.HostPort, Bytes: len(body)}, nil
	}
	return f, &seen
}
```

- [ ] **Step 2: Add a typed forwarding submit test**

In `tui/app_test.go`, add this test after `TestSubmitFetchesParsedTargetAndBlurs`:

```go
func TestSubmitFetchesForwardedTarget(t *testing.T) {
	fetch, seen := fetchTargetRecorder("Plan: forwarded\n")
	m := newApp(fetch, colorprofile.NoTTY)
	m.input.SetValue("alice@tilde.team@thebackupbox.net")

	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = step.(appModel)
	if m.inputFocused {
		t.Fatal("submit should blur the input to content")
	}
	if cmd == nil {
		t.Fatal("submit should return a fetch command")
	}
	runCmds(cmd)

	if len(*seen) != 1 {
		t.Fatalf("fetched %d targets, want 1", len(*seen))
	}
	got := (*seen)[0]
	if got.HostPort != "thebackupbox.net:79" {
		t.Fatalf("HostPort = %q, want thebackupbox.net:79", got.HostPort)
	}
	if got.QueryLine() != "alice@tilde.team" {
		t.Fatalf("QueryLine = %q, want alice@tilde.team", got.QueryLine())
	}
	if got.Raw != "alice@tilde.team@thebackupbox.net" {
		t.Fatalf("Raw = %q, want alice@tilde.team@thebackupbox.net", got.Raw)
	}
}
```

- [ ] **Step 3: Add TUI refusal tests for server-supplied forwarding**

In `tui/app_test.go`, add these tests near the existing server-supplied target tests:

```go
func TestDrillServerSuppliedForwardedTargetFlashesRefusal(t *testing.T) {
	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@thebackupbox.net")
	users := []User{{Login: "alice", Target: "alice@tilde.team@thebackupbox.net"}}
	m.history = []histNode{{entry: Entry{Target: host}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.state = stateList
	m.inputFocused = false

	step, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got := step.(appModel)

	if cmd == nil {
		t.Fatal("drill refusal should return a clear-flash command")
	}
	if got.loading {
		t.Fatal("server-supplied forwarded target must not start loading")
	}
	if got.flash != finger.ErrServerForwarding.Error() {
		t.Fatalf("flash = %q, want %q", got.flash, finger.ErrServerForwarding.Error())
	}
}

func TestCopyServerSuppliedForwardedTargetFlashesRefusal(t *testing.T) {
	var copied string
	setClipboard = func(s string) tea.Cmd { copied = s; return nil }
	defer func() { setClipboard = tea.SetClipboard }()

	m := newApp(stubFetch(t), colorprofile.NoTTY)
	host := hostTarget(t, "@thebackupbox.net")
	users := []User{{Login: "alice", Target: "alice@tilde.team@thebackupbox.net"}}
	m.history = []histNode{{entry: Entry{Target: host}, state: stateList}}
	m.pos = 0
	m.listReady = true
	m.list = newList(m.common, host, users)
	m.list.list.Select(0)
	m.state = stateList
	m.inputFocused = false

	step, _ := m.Update(tea.KeyPressMsg{Code: 'y'})
	got := step.(appModel)

	if copied != "" {
		t.Fatalf("copied = %q, want empty", copied)
	}
	if got.flash != finger.ErrServerForwarding.Error() {
		t.Fatalf("flash = %q, want %q", got.flash, finger.ErrServerForwarding.Error())
	}
}
```

- [ ] **Step 4: Run TUI tests and confirm refusal path fails before app changes**

Run:

```bash
go test ./tui/ -run 'TestSubmitFetchesForwardedTarget|TestDrillServerSuppliedForwardedTargetFlashesRefusal|TestCopyServerSuppliedForwardedTargetFlashesRefusal' -count=1 -v
```

Expected: `TestSubmitFetchesForwardedTarget` may pass after Tasks 1-3; the two server-supplied refusal tests should FAIL because `drill` and `copyAddress` currently swallow parse errors or map them to `nothing to copy`.

- [ ] **Step 5: Import `errors` in `tui/app.go`**

In `tui/app.go`, add `errors` to the import block:

```go
import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
```

- [ ] **Step 6: Surface server-supplied forwarding errors in drill**

In `tui/app.go`, replace the parse-error block in `drill`:

```go
if err != nil {
	return true, m, nil
}
```

with:

```go
if err != nil {
	if errors.Is(err, finger.ErrServerForwarding) {
		return true, m, m.setFlash(err.Error())
	}
	return true, m, nil
}
```

- [ ] **Step 7: Surface server-supplied forwarding errors in copy**

In `tui/app.go`, replace the server-supplied branch inside `copyAddress`:

```go
if t, err := finger.ParseTargetPinned(sel.target); err == nil {
	addr = t.Raw
}
```

with:

```go
t, err := finger.ParseTargetPinned(sel.target)
if err != nil {
	if errors.Is(err, finger.ErrServerForwarding) {
		return m.setFlash(err.Error())
	}
} else {
	addr = t.Raw
}
```

- [ ] **Step 8: Route forwarded host/list queries as list-capable**

In `tui/app.go`, replace `shouldOpenList`:

```go
func shouldOpenList(entry Entry) bool {
	return entry.Target.User == "" ||
		(entry.Target.User == "ring" && strings.HasPrefix(entry.Target.HostPort, "thebackupbox.net:"))
}
```

with:

```go
func shouldOpenList(entry Entry) bool {
	return entry.Target.HostQuery() ||
		(entry.Target.QueryLine() == "ring" && strings.HasPrefix(entry.Target.HostPort, "thebackupbox.net:"))
}
```

- [ ] **Step 9: Use `QueryLine` in breadcrumbs**

In `tui/statusbar.go`, replace:

```go
return "@" + h, t.User
```

with:

```go
return "@" + h, t.QueryLine()
```

- [ ] **Step 10: Run focused TUI tests**

Run:

```bash
go test ./tui/ -run 'TestSubmitFetchesForwardedTarget|TestDrillServerSuppliedForwardedTargetFlashesRefusal|TestCopyServerSuppliedForwardedTargetFlashesRefusal|TestDrillServerSuppliedTargetPinnedToPort79|TestCopyAddressPinsServerTarget' -count=1 -v
```

Expected: PASS.

- [ ] **Step 11: Commit this task if commit approval has been granted**

Run only if the user has explicitly approved commits:

```bash
git add tui/app.go tui/statusbar.go tui/app_test.go
git commit -m "feat(tui): handle one-relay forwarding safely"
```

Expected: a conventional commit with no co-author or AI trailers.

---

### Task 5: Final Verification and Review

**Files:**
- Verify: all modified files

- [ ] **Step 1: Run package tests**

Run:

```bash
go test ./finger/ ./tui/ -count=1
```

Expected: PASS.

- [ ] **Step 2: Run formatting**

Run:

```bash
make fmt
```

Expected: no errors. If `make fmt` changes files, inspect the diff before continuing.

- [ ] **Step 3: Run the full CI gate**

Run:

```bash
make check
```

Expected: PASS for `go vet ./...`, `gofmt -l .` emptiness check, `golangci-lint run ./...`, and `go test ./... -race`.

- [ ] **Step 4: Inspect the diff**

Run:

```bash
git diff -- finger/query.go finger/client.go finger/query_test.go finger/client_test.go finger/fuzz_test.go tui/app.go tui/statusbar.go tui/app_test.go docs/superpowers/specs/2026-06-05-one-relay-forwarding-design.md docs/superpowers/plans/2026-06-05-one-relay-forwarding.md
```

Expected:

- `Target.Query` is the only target-model expansion.
- `finger.Query` writes `QueryLine()`.
- Typed one-relay forwarding is accepted.
- Server-supplied forwarding is rejected with `ErrServerForwarding`.
- URL forwarding is rejected with the deferred-feature error.
- No unrelated files are changed except pre-existing user edits.

- [ ] **Step 5: Commit this task if commit approval has been granted**

Run only if the user has explicitly approved commits and previous task commits were not made:

```bash
git add finger/query.go finger/client.go finger/query_test.go finger/client_test.go finger/fuzz_test.go tui/app.go tui/statusbar.go tui/app_test.go docs/superpowers/specs/2026-06-05-one-relay-forwarding-design.md docs/superpowers/plans/2026-06-05-one-relay-forwarding.md
git commit -m "feat(finger): support one-relay forwarding"
```

Expected: a conventional commit with no co-author or AI trailers.
