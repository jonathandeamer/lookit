# lookit MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Phase 1 CLI MVP for `lookit`: a polished one-shot finger client (`lookit user@host`, `lookit @host`, `lookit user@host:port`) with chrome + structured-field-highlighting output and graceful degradation on limited terminals.

**Architecture:** Three packages with strict boundaries: `finger/` handles networking only (knows nothing about styling), `render/` handles output styling only (knows nothing about networking), and `main.go` is thin wiring (~50 lines). The split is chosen so Phase 2's TUI can reuse `finger/` and `render/` unchanged.

**Tech Stack:** Go (1.22+), `lipgloss` for styling, `colorprofile` for terminal capability detection + color downsampling. Standard library for networking, arg parsing, and JSON. No fang yet (added in Phase 2). No TUI yet.

**Repo:** `/Users/jonathan/lookit/` (git already initialized; the spec is committed at `docs/superpowers/specs/2026-05-28-lookit-design.md`).

---

## File Structure

```
lookit/
├── .github/workflows/ci.yml         # test + vet + gofmt CI
├── .gitignore                       # binaries, dist/
├── README.md                        # user-facing intro + install + usage
├── go.mod
├── go.sum
├── main.go                          # arg parse → finger.Query → render.Render → print
├── finger/
│   ├── query.go                     # Target type + ParseTarget
│   ├── client.go                    # Meta type + Query (dial, send, read)
│   └── client_test.go               # in-process fake server tests
└── render/
    ├── theme.go                     # Theme struct: lipgloss styles built per colorprofile
    ├── chrome.go                    # renderHeader, renderFooter
    ├── fields.go                    # field-prefix detection + label highlighting
    ├── render.go                    # Render() entry point that composes the above
    ├── render_test.go               # golden tests with -update flag
    └── testdata/
        ├── basic.input
        ├── basic.truecolor.golden
        ├── basic.notty.golden
        ├── ascii-art.input
        ├── ascii-art.truecolor.golden
        ├── empty.input
        ├── empty.truecolor.golden
        ├── truncated.input
        ├── truncated.truecolor.golden
        ├── timeout.input
        └── timeout.truecolor.golden
```

**Types (locked here so all tasks reference the same shapes):**

```go
// finger/query.go
type Target struct {
    User     string // empty for the "@host" form
    HostPort string // always "host:port" — port defaults to "79" if not in input
    Raw      string // original arg, used in chrome (e.g. "alice@plan.cat")
}

func ParseTarget(arg string) (Target, error)

// finger/client.go
type Meta struct {
    Addr      string        // resolved "host:port"
    Elapsed   time.Duration // wall clock from dial start to read end
    Bytes     int           // bytes of body returned (after \r\n normalization)
    Truncated bool          // true only if 1 MiB cap or 30s read deadline triggered
}

func Query(ctx context.Context, t Target) (body []byte, meta Meta, err error)

// render/render.go
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string
```

---

## Task 1: Bootstrap the Go module

**Files:**
- Create: `/Users/jonathan/lookit/go.mod`
- Create: `/Users/jonathan/lookit/.gitignore`
- Create: `/Users/jonathan/lookit/README.md`

- [ ] **Step 1: Initialize the module**

The user's GitHub handle is unconfirmed. Use `github.com/jonathandeamer/lookit` as the module path; if wrong, the user can run `go mod edit -module github.com/<handle>/lookit` later and find-replace imports.

Run:
```bash
cd /Users/jonathan/lookit
go mod init github.com/jonathandeamer/lookit
```

Expected: `go.mod` created with `module github.com/jonathandeamer/lookit` and `go 1.22` (or newer — whatever the local toolchain is).

- [ ] **Step 2: Add the runtime dependencies**

Run:
```bash
cd /Users/jonathan/lookit
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/colorprofile@latest
```

Expected: `go.sum` created. `go.mod` lists both as `require`.

- [ ] **Step 3: Create .gitignore**

Create `/Users/jonathan/lookit/.gitignore`:

```
# Binary
/lookit

# Build artifacts
/dist/

# OS junk
.DS_Store
```

- [ ] **Step 4: Create README skeleton**

Create `/Users/jonathan/lookit/README.md`:

```markdown
# lookit

A finger client for the modern terminal.

Lookit talks RFC 1288 finger over TCP/79 and renders the response with chrome and structured field highlighting. Built with [Charm](https://charm.sh) tools.

## Status

Phase 1 (CLI MVP) — under construction. See [design spec](docs/superpowers/specs/2026-05-28-lookit-design.md).

## Install

```bash
go install github.com/jonathandeamer/lookit@latest
```

## Usage

```bash
lookit alice@plan.cat
lookit @tilde.team
lookit alice@example.com:7979
```

## License

TBD.
```

- [ ] **Step 5: Commit**

Run:
```bash
cd /Users/jonathan/lookit
git add go.mod go.sum .gitignore README.md
git commit -m "chore: bootstrap go module and project skeleton"
```

---

## Task 2: `finger.ParseTarget` — argument parsing

**Files:**
- Create: `/Users/jonathan/lookit/finger/query.go`
- Create: `/Users/jonathan/lookit/finger/query_test.go`

- [ ] **Step 1: Write the failing test**

Create `/Users/jonathan/lookit/finger/query_test.go`:

```go
package finger

import "testing"

func TestParseTarget(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		want     Target
		wantErr  bool
	}{
		{
			name:  "user@host",
			input: "alice@plan.cat",
			want:  Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"},
		},
		{
			name:  "@host (server query)",
			input: "@tilde.team",
			want:  Target{User: "", HostPort: "tilde.team:79", Raw: "@tilde.team"},
		},
		{
			name:  "user@host:port",
			input: "alice@example.com:7979",
			want:  Target{User: "alice", HostPort: "example.com:7979", Raw: "alice@example.com:7979"},
		},
		{
			name:  "@host:port",
			input: "@example.com:7979",
			want:  Target{User: "", HostPort: "example.com:7979", Raw: "@example.com:7979"},
		},
		{
			name:    "missing @",
			input:   "alice",
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "@ with no host",
			input:   "alice@",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTarget(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseTarget(%q): expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTarget(%q): unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseTarget(%q):\n  got:  %#v\n  want: %#v", tc.input, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/...
```

Expected: build failure — `undefined: Target`, `undefined: ParseTarget`. That's the right kind of fail.

- [ ] **Step 3: Implement `Target` + `ParseTarget`**

Create `/Users/jonathan/lookit/finger/query.go`:

```go
// Package finger implements an RFC 1288 finger client.
package finger

import (
	"errors"
	"strings"
)

// Target identifies a finger query: an optional user and a host:port pair.
type Target struct {
	User     string // empty for the bare "@host" form
	HostPort string // always "host:port"; port defaults to "79"
	Raw      string // original argument string, e.g. "alice@plan.cat"
}

// ParseTarget parses one of these forms:
//
//	user@host
//	@host
//	user@host:port
//	@host:port
//
// Returns an error for missing "@", empty input, or empty host.
func ParseTarget(arg string) (Target, error) {
	if arg == "" {
		return Target{}, errors.New("empty target")
	}
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
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/...
```

Expected: `ok  github.com/jonathandeamer/lookit/finger  ...`

- [ ] **Step 5: Commit**

Run:
```bash
cd /Users/jonathan/lookit
git add finger/query.go finger/query_test.go
git commit -m "feat(finger): parse user@host[:port] and @host targets"
```

---

## Task 3: `finger.Query` — happy path against an in-process fake server

**Files:**
- Create: `/Users/jonathan/lookit/finger/client.go`
- Create: `/Users/jonathan/lookit/finger/client_test.go`

- [ ] **Step 1: Write the test (with fake-server helper)**

Create `/Users/jonathan/lookit/finger/client_test.go`:

```go
package finger

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"
)

// fakeServer accepts one connection, reads one CRLF-terminated line,
// passes it to handler, and writes the response. handler returns the
// response bytes to send, or nil to just close the connection.
type fakeServer struct {
	t       *testing.T
	ln      net.Listener
	addr    string
	gotLine string
}

func newFakeServer(t *testing.T, handler func(line string) []byte) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	fs := &fakeServer{t: t, ln: ln, addr: ln.Addr().String()}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		fs.gotLine = line
		if body := handler(line); body != nil {
			_, _ = conn.Write(body)
		}
	}()
	return fs
}

func TestQuery_UserHappyPath(t *testing.T) {
	fs := newFakeServer(t, func(line string) []byte {
		return []byte("Login: alice\r\nName: Alice\r\nPlan:\r\nhello world\r\n")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	body, meta, err := Query(ctx, Target{User: "alice", HostPort: fs.addr, Raw: "alice@" + fs.addr})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if fs.gotLine != "alice\r\n" {
		t.Errorf("server received %q, want %q", fs.gotLine, "alice\r\n")
	}
	want := "Login: alice\nName: Alice\nPlan:\nhello world\n"
	if string(body) != want {
		t.Errorf("body:\n  got:  %q\n  want: %q", body, want)
	}
	if meta.Truncated {
		t.Errorf("Truncated = true, want false")
	}
	if meta.Bytes != len(body) {
		t.Errorf("Bytes = %d, want %d", meta.Bytes, len(body))
	}
	if meta.Addr != fs.addr {
		t.Errorf("Addr = %q, want %q", meta.Addr, fs.addr)
	}
	if meta.Elapsed <= 0 {
		t.Errorf("Elapsed = %v, want > 0", meta.Elapsed)
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_UserHappyPath
```

Expected: build failure — `undefined: Query`, `undefined: Meta`.

- [ ] **Step 3: Implement `Meta` + `Query` (minimal — just happy path)**

Create `/Users/jonathan/lookit/finger/client.go`:

```go
package finger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"
)

// Meta describes the outcome of a Query that produced a body.
type Meta struct {
	Addr      string
	Elapsed   time.Duration
	Bytes     int
	Truncated bool
}

const (
	connectTimeout = 10 * time.Second
	readTimeout    = 30 * time.Second
	maxBodyBytes   = 1 << 20 // 1 MiB
)

// Query performs a single finger query against t.HostPort.
// On read deadline or size cap, returns the partial body with
// Meta.Truncated = true and a non-nil err.
func Query(ctx context.Context, t Target) ([]byte, Meta, error) {
	start := time.Now()
	meta := Meta{Addr: t.HostPort}

	dialCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", t.HostPort)
	if err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("dial %s: %w", t.HostPort, err)
	}
	defer conn.Close()

	// Send the query line: "<user>\r\n" (user may be empty for @host).
	if _, err := fmt.Fprintf(conn, "%s\r\n", t.User); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("write query: %w", err)
	}

	// Read until EOF.
	raw, err := io.ReadAll(conn)
	if err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("read response: %w", err)
	}

	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	meta.Bytes = len(body)
	meta.Elapsed = time.Since(start)
	return body, meta, nil
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_UserHappyPath -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/jonathan/lookit
git add finger/client.go finger/client_test.go
git commit -m "feat(finger): Query happy path with CRLF normalization"
```

---

## Task 4: `finger.Query` — `@host` (server query) form

**Files:**
- Modify: `/Users/jonathan/lookit/finger/client_test.go`

- [ ] **Step 1: Add failing test**

Append to `/Users/jonathan/lookit/finger/client_test.go`:

```go
func TestQuery_ServerForm(t *testing.T) {
	fs := newFakeServer(t, func(line string) []byte {
		return []byte("Welcome to plan.cat\r\nUsers: alice, bob\r\n")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	body, _, err := Query(ctx, Target{User: "", HostPort: fs.addr, Raw: "@" + fs.addr})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// The "@host" form sends just "\r\n" (empty user).
	if fs.gotLine != "\r\n" {
		t.Errorf("server received %q, want %q", fs.gotLine, "\r\n")
	}
	want := "Welcome to plan.cat\nUsers: alice, bob\n"
	if string(body) != want {
		t.Errorf("body:\n  got:  %q\n  want: %q", body, want)
	}
}
```

- [ ] **Step 2: Run the test**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_ServerForm -v
```

Expected: PASS (current implementation handles `User == ""` correctly because `fmt.Fprintf(conn, "%s\r\n", "")` writes `"\r\n"`).

- [ ] **Step 3: Commit**

```bash
cd /Users/jonathan/lookit
git add finger/client_test.go
git commit -m "test(finger): lock in @host server-query form"
```

---

## Task 5: `finger.Query` — 30s read deadline

**Files:**
- Modify: `/Users/jonathan/lookit/finger/client.go`
- Modify: `/Users/jonathan/lookit/finger/client_test.go`

- [ ] **Step 1: Write the failing test**

The default 30s deadline is too long for a unit test. We need to make `readTimeout` injectable. Best way: add an unexported `queryOpts` and a test helper.

Add to `/Users/jonathan/lookit/finger/client.go` (replace the constants block + Query signature):

```go
const (
	defaultConnectTimeout = 10 * time.Second
	defaultReadTimeout    = 30 * time.Second
	maxBodyBytes          = 1 << 20 // 1 MiB
)

// queryOpts are tuneable knobs used by tests. Zero values mean "use defaults".
type queryOpts struct {
	connectTimeout time.Duration
	readTimeout    time.Duration
}

// queryWith is Query with overrides. The exported Query forwards with zero opts.
func queryWith(ctx context.Context, t Target, opts queryOpts) ([]byte, Meta, error) {
	// ... will be filled in below
	panic("not yet")
}

func Query(ctx context.Context, t Target) ([]byte, Meta, error) {
	return queryWith(ctx, t, queryOpts{})
}
```

Then append to `client_test.go`:

```go
func TestQuery_ReadDeadline(t *testing.T) {
	// Server accepts but never writes — should hit the read deadline.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Hold the connection open without writing.
		time.Sleep(2 * time.Second)
		conn.Close()
	}()

	ctx := context.Background()
	body, meta, err := queryWith(ctx, Target{HostPort: ln.Addr().String()}, queryOpts{
		readTimeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !meta.Truncated {
		t.Errorf("Truncated = false, want true on timeout")
	}
	if body == nil {
		t.Logf("body is nil — acceptable; the point is Truncated=true")
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_ReadDeadline -v
```

Expected: build failure or panic (`queryWith` is `panic("not yet")`).

- [ ] **Step 3: Implement `queryWith` with deadline handling**

Replace the body of `queryWith` in `/Users/jonathan/lookit/finger/client.go`:

```go
func queryWith(ctx context.Context, t Target, opts queryOpts) ([]byte, Meta, error) {
	if opts.connectTimeout == 0 {
		opts.connectTimeout = defaultConnectTimeout
	}
	if opts.readTimeout == 0 {
		opts.readTimeout = defaultReadTimeout
	}

	start := time.Now()
	meta := Meta{Addr: t.HostPort}

	dialCtx, cancel := context.WithTimeout(ctx, opts.connectTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", t.HostPort)
	if err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("dial %s: %w", t.HostPort, err)
	}
	defer conn.Close()

	// Overall read deadline. context.WithTimeout alone does NOT interrupt
	// a blocking net.Conn.Read; SetDeadline does.
	if err := conn.SetDeadline(time.Now().Add(opts.readTimeout)); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("set deadline: %w", err)
	}

	if _, err := fmt.Fprintf(conn, "%s\r\n", t.User); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("write query: %w", err)
	}

	raw, readErr := io.ReadAll(conn)
	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	meta.Bytes = len(body)
	meta.Elapsed = time.Since(start)

	if readErr != nil {
		// Timeout? Treat as truncated. Other errors propagate as-is.
		if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
			meta.Truncated = true
			return body, meta, fmt.Errorf("read response timed out after %s: %w", opts.readTimeout, readErr)
		}
		return body, meta, fmt.Errorf("read response: %w", readErr)
	}
	return body, meta, nil
}
```

- [ ] **Step 4: Run the failing-deadline test, verify it passes**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_ReadDeadline -v
```

Expected: PASS.

- [ ] **Step 5: Run the whole package to confirm no regressions**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add finger/client.go finger/client_test.go
git commit -m "feat(finger): 30s read deadline via SetDeadline"
```

---

## Task 6: `finger.Query` — 1 MiB size cap

**Files:**
- Modify: `/Users/jonathan/lookit/finger/client.go`
- Modify: `/Users/jonathan/lookit/finger/client_test.go`

- [ ] **Step 1: Write the failing test**

Append to `client_test.go`:

```go
func TestQuery_SizeCap(t *testing.T) {
	// Server streams a body larger than maxBodyBytes (1 MiB).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = bufio.NewReader(conn).ReadString('\n')
		// 2 MiB of 'x'
		buf := make([]byte, 64*1024)
		for i := range buf {
			buf[i] = 'x'
		}
		for written := 0; written < 2<<20; written += len(buf) {
			if _, err := conn.Write(buf); err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body, meta, err := Query(ctx, Target{HostPort: ln.Addr().String()})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !meta.Truncated {
		t.Errorf("Truncated = false, want true after exceeding cap")
	}
	if len(body) != maxBodyBytes {
		t.Errorf("len(body) = %d, want %d", len(body), maxBodyBytes)
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_SizeCap -v
```

Expected: FAIL — body is full 2 MiB, not capped.

- [ ] **Step 3: Add the size cap to `queryWith`**

In `/Users/jonathan/lookit/finger/client.go`, replace the `io.ReadAll(conn)` line and the subsequent body handling with a capped reader. The relevant section becomes:

```go
	// Read up to maxBodyBytes + 1 so we can detect overflow.
	lr := &io.LimitedReader{R: conn, N: maxBodyBytes + 1}
	raw, readErr := io.ReadAll(lr)

	truncatedByCap := false
	if len(raw) > maxBodyBytes {
		raw = raw[:maxBodyBytes]
		truncatedByCap = true
	}

	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	meta.Bytes = len(body)
	meta.Elapsed = time.Since(start)

	if truncatedByCap {
		meta.Truncated = true
		// readErr may be non-nil (timeout) or nil — either way, cap wins.
		return body, meta, nil
	}

	if readErr != nil {
		if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
			meta.Truncated = true
			return body, meta, fmt.Errorf("read response timed out after %s: %w", opts.readTimeout, readErr)
		}
		return body, meta, fmt.Errorf("read response: %w", readErr)
	}
	return body, meta, nil
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_SizeCap -v
```

Expected: PASS.

- [ ] **Step 5: Run whole package**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add finger/client.go finger/client_test.go
git commit -m "feat(finger): cap response body at 1 MiB"
```

---

## Task 7: `finger.Query` — context cancellation interrupts blocking read

**Files:**
- Modify: `/Users/jonathan/lookit/finger/client.go`
- Modify: `/Users/jonathan/lookit/finger/client_test.go`

- [ ] **Step 1: Write the failing test**

Append to `client_test.go`:

```go
func TestQuery_ContextCancel(t *testing.T) {
	// Server accepts and stalls; we cancel the context.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		time.Sleep(5 * time.Second)
		conn.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, _, err = Query(ctx, Target{HostPort: ln.Addr().String()})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Query took %v after cancel; want < 2s (cancel should close conn promptly)", elapsed)
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_ContextCancel -v
```

Expected: FAIL — without ctx-cancel handling, the test sits until the 30s default read deadline (or until the server closes at 5s).

- [ ] **Step 3: Add ctx-cancel goroutine**

In `/Users/jonathan/lookit/finger/client.go`, in `queryWith`, right after `defer conn.Close()` and before `SetDeadline`, add:

```go
	// Propagate caller-ctx cancellation to the connection. A blocking
	// net.Conn.Read does not observe context.Done() on its own.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_ContextCancel -v
```

Expected: PASS within ~100ms.

- [ ] **Step 5: Run whole package**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add finger/client.go finger/client_test.go
git commit -m "feat(finger): propagate context cancel to conn close"
```

---

## Task 8: `finger.Query` — partial response on EOF is NOT truncation

**Files:**
- Modify: `/Users/jonathan/lookit/finger/client_test.go`

This locks in the EOF semantics from the spec: a server that closes without a trailing newline is fine, not "truncated."

- [ ] **Step 1: Add the test**

Append to `client_test.go`:

```go
func TestQuery_EOFMidLineNotTruncated(t *testing.T) {
	// Server sends partial line (no final \r\n) then closes.
	fs := newFakeServer(t, func(line string) []byte {
		return []byte("Login: alice\r\nName: Alice\r\nno trailing newline here")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, meta, err := Query(ctx, Target{HostPort: fs.addr})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if meta.Truncated {
		t.Errorf("Truncated = true, want false for normal TCP-close EOF")
	}
	want := "Login: alice\nName: Alice\nno trailing newline here"
	if string(body) != want {
		t.Errorf("body:\n  got:  %q\n  want: %q", body, want)
	}
}
```

- [ ] **Step 2: Run the test, verify it passes (no impl change needed)**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_EOFMidLineNotTruncated -v
```

Expected: PASS. `io.ReadAll` returns nil error on natural EOF, so `meta.Truncated` stays false. This test is a regression guard.

- [ ] **Step 3: Add the Latin-1 / binary pass-through test**

Append to `client_test.go`:

```go
func TestQuery_BinaryBytesPreserved(t *testing.T) {
	// Server emits a high-bit byte (Latin-1 'é' = 0xE9) that's not valid UTF-8.
	fs := newFakeServer(t, func(line string) []byte {
		return []byte{'P', 'l', 'a', 'n', ':', '\r', '\n', 'c', 'a', 'f', 0xE9, '\r', '\n'}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, _, err := Query(ctx, Target{HostPort: fs.addr})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []byte{'P', 'l', 'a', 'n', ':', '\n', 'c', 'a', 'f', 0xE9, '\n'}
	if !bytes.Equal(body, want) {
		t.Errorf("body:\n  got:  % x\n  want: % x", body, want)
	}
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -run TestQuery_BinaryBytesPreserved -v
```

Expected: PASS. Our implementation does byte-level `bytes.ReplaceAll` and never decodes as UTF-8.

- [ ] **Step 5: Run whole package + vet**

Run:
```bash
cd /Users/jonathan/lookit
go test ./finger/... -v
go vet ./finger/...
```

Expected: all PASS, no vet warnings.

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add finger/client_test.go
git commit -m "test(finger): EOF mid-line is not truncation; binary bytes preserved"
```

---

## Task 9: `render` — first golden test (basic response, TrueColor profile)

**Files:**
- Create: `/Users/jonathan/lookit/render/render.go`
- Create: `/Users/jonathan/lookit/render/chrome.go`
- Create: `/Users/jonathan/lookit/render/theme.go`
- Create: `/Users/jonathan/lookit/render/render_test.go`
- Create: `/Users/jonathan/lookit/render/testdata/basic.input`
- Create: `/Users/jonathan/lookit/render/testdata/basic.truecolor.golden`

- [ ] **Step 1: Create the test data input**

Create `/Users/jonathan/lookit/render/testdata/basic.input`:

```
Login: alice
Name: Alice Example
Directory: /home/alice
Shell: /bin/zsh
On since Mon Mar 10 09:14 (PST) on tty1
Plan:
This is my plan for today.
- finish lookit MVP
- have a snack
```

- [ ] **Step 2: Write the failing test with -update support**

Create `/Users/jonathan/lookit/render/render_test.go`:

```go
package render

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

var update = flag.Bool("update", false, "update golden files")

// loadInput reads testdata/<name>.input.
func loadInput(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".input"))
	if err != nil {
		t.Fatalf("read input %s: %v", name, err)
	}
	return b
}

// compareGolden compares got against testdata/<name>.<profile>.golden.
// With -update, writes got to the golden file.
func compareGolden(t *testing.T, name, profile, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+"."+profile+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated golden %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if got != string(want) {
		t.Errorf("output differs from golden %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}

func TestRender_BasicTrueColor(t *testing.T) {
	body := loadInput(t, "basic")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{
		Addr:    "plan.cat:79",
		Elapsed: 123 * time.Millisecond,
		Bytes:   len(body),
	}
	got := Render(target, body, meta, nil, colorprofile.TrueColor)
	compareGolden(t, "basic", "truecolor", got)
}
```

- [ ] **Step 3: Run the test, verify it fails**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/...
```

Expected: build failure — `undefined: Render`.

- [ ] **Step 4: Implement the `render` package skeleton**

Create `/Users/jonathan/lookit/render/theme.go`:

```go
// Package render formats finger responses for terminal display.
package render

import (
	"image/color"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds the styles used by Render. Build one per profile.
type Theme struct {
	Profile colorprofile.Profile

	Arrow   lipgloss.Style // "➜"
	Target  lipgloss.Style // "alice@plan.cat"
	Latency lipgloss.Style // "123ms"
	Sparkle lipgloss.Style // success indicator
	Footer  lipgloss.Style // bytes / elapsed line (dim)
	Warning lipgloss.Style // yellow notices like "truncated"
	Field   lipgloss.Style // "Login:" labels
	ErrLine lipgloss.Style // red error lines

	NoColor bool // true if the profile is Ascii or NoTTY
}

// pink, cyan, gold, red, dim — base palette (truecolor source values).
var (
	colPink = color.RGBA{0xff, 0x6f, 0xd5, 0xff}
	colCyan = color.RGBA{0x6b, 0xe1, 0xff, 0xff}
	colGold = color.RGBA{0xff, 0xd0, 0x6b, 0xff}
	colRed  = color.RGBA{0xff, 0x6b, 0x6b, 0xff}
	colDim  = color.RGBA{0x80, 0x80, 0x80, 0xff}
)

// NewTheme builds a Theme appropriate for the given profile. On Ascii/NoTTY
// profiles, returns a no-color theme that still preserves spacing.
func NewTheme(p colorprofile.Profile) Theme {
	noColor := p == colorprofile.Ascii || p == colorprofile.NoTTY
	style := func(c color.Color) lipgloss.Style {
		if noColor {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(toHex(p.Convert(c))))
	}
	return Theme{
		Profile: p,
		NoColor: noColor,
		Arrow:   style(colPink).Bold(true),
		Target:  style(colPink).Bold(true),
		Latency: style(colDim),
		Sparkle: style(colGold),
		Footer:  style(colDim),
		Warning: style(colGold),
		Field:   style(colCyan).Bold(true),
		ErrLine: style(colRed),
	}
}

// toHex formats a color.Color as "#RRGGBB" for lipgloss.Color.
func toHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	const hex = "0123456789abcdef"
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	rb, gb, bb := byte(r>>8), byte(g>>8), byte(b>>8)
	out[1] = hex[rb>>4]
	out[2] = hex[rb&0xf]
	out[3] = hex[gb>>4]
	out[4] = hex[gb&0xf]
	out[5] = hex[bb>>4]
	out[6] = hex[bb&0xf]
	return string(out)
}
```

Create `/Users/jonathan/lookit/render/chrome.go`:

```go
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/jonathandeamer/lookit/finger"
)

// renderHeader writes the leading "➜ alice@plan.cat   123ms ✦" line.
func renderHeader(theme Theme, t finger.Target, meta finger.Meta, success bool) string {
	arrow := theme.Arrow.Render("➜")
	target := theme.Target.Render(t.Raw)
	latency := theme.Latency.Render(fmtElapsed(meta.Elapsed))
	parts := []string{arrow, target, latency}
	if success {
		parts = append(parts, theme.Sparkle.Render("✦"))
	}
	return strings.Join(parts, " ") + "\n"
}

// renderFooter writes the trailing "1.2 KiB · 123ms" line, plus any notices.
func renderFooter(theme Theme, meta finger.Meta, notice string) string {
	stats := fmt.Sprintf("%s · %s", fmtBytes(meta.Bytes), fmtElapsed(meta.Elapsed))
	line := theme.Footer.Render(stats)
	if notice != "" {
		// Per the spec, the truncation notice is yellow (Warning) to set it
		// apart from the regular dim footer stats.
		line += " " + theme.Footer.Render("·") + " " + theme.Warning.Render(notice)
	}
	return "\n" + line + "\n"
}

func fmtElapsed(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func fmtBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	}
}
```

Create `/Users/jonathan/lookit/render/render.go`:

```go
package render

import (
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

// Render produces a styled string representation of a finger query result.
// queryErr is the error returned by finger.Query, if any. profile is the
// detected terminal color profile (use colorprofile.NoTTY for plain output).
func Render(t finger.Target, body []byte, meta finger.Meta, queryErr error, profile colorprofile.Profile) string {
	theme := NewTheme(profile)
	var sb strings.Builder

	success := queryErr == nil
	sb.WriteString(renderHeader(theme, t, meta, success))

	if len(body) == 0 && queryErr == nil {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteString("\n")
	} else {
		sb.Write(body)
		if len(body) > 0 && body[len(body)-1] != '\n' {
			sb.WriteString("\n")
		}
	}

	notice := ""
	if meta.Truncated {
		notice = "truncated"
	}
	sb.WriteString(renderFooter(theme, meta, notice))

	if queryErr != nil {
		sb.WriteString(theme.ErrLine.Render(queryErr.Error()))
		sb.WriteString("\n")
	}

	return sb.String()
}
```

- [ ] **Step 5: Run the test with -update to create the initial golden**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -run TestRender_BasicTrueColor -update -v
```

Expected: PASS, plus a log line "updated golden testdata/basic.truecolor.golden".

- [ ] **Step 6: Eyeball the golden**

Run:
```bash
cd /Users/jonathan/lookit
cat render/testdata/basic.truecolor.golden
```

Expected: text content matches the input visually (you'll see ANSI escapes around `➜`, `alice@plan.cat`, `123ms`, the closing footer, etc.). The body should appear verbatim except for the trailing newline normalization. The `Login:` / `Name:` / `Plan:` etc. labels are NOT highlighted yet — that comes in Task 11.

- [ ] **Step 7: Re-run without -update to confirm golden matches**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/jonathan/lookit
git add render/ go.mod go.sum
git commit -m "feat(render): theme + chrome + first golden test (truecolor)"
```

---

## Task 10: `render` — Ascii / NoTTY profile produces no ANSI escapes

**Files:**
- Modify: `/Users/jonathan/lookit/render/render_test.go`
- Create: `/Users/jonathan/lookit/render/testdata/basic.notty.golden`

- [ ] **Step 1: Add the failing test**

Append to `render_test.go`:

```go
import "strings"

func TestRender_BasicNoTTY(t *testing.T) {
	body := loadInput(t, "basic")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}
	got := Render(target, body, meta, nil, colorprofile.NoTTY)
	compareGolden(t, "basic", "notty", got)
}

func TestRender_NoTTY_HasNoANSI(t *testing.T) {
	body := loadInput(t, "basic")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 123 * time.Millisecond, Bytes: len(body)}
	got := Render(target, body, meta, nil, colorprofile.NoTTY)
	if strings.Contains(got, "\x1b[") {
		t.Errorf("NoTTY output contains ANSI escape sequence; output:\n%s", got)
	}
}
```

(If the existing import block doesn't include `"strings"`, merge it in rather than adding a second import.)

- [ ] **Step 2: Run, verify the golden-creation case fails and the no-ANSI case passes (or fails — we'll see)**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -run TestRender_BasicNoTTY -v
```

Expected: FAIL because `basic.notty.golden` doesn't exist yet.

- [ ] **Step 3: Create the NoTTY golden with -update**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -run TestRender_BasicNoTTY -update -v
```

Expected: PASS. Golden file written.

- [ ] **Step 4: Inspect the NoTTY golden**

Run:
```bash
cd /Users/jonathan/lookit
cat render/testdata/basic.notty.golden
```

Expected: plain text, no ANSI escapes anywhere. Looks like a slightly chrome'd version of the input.

- [ ] **Step 5: Run all render tests**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -v
```

Expected: all PASS, including `TestRender_NoTTY_HasNoANSI`.

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add render/render_test.go render/testdata/basic.notty.golden
git commit -m "test(render): NoTTY profile emits no ANSI escapes"
```

---

## Task 11: `render` — field highlighting

**Files:**
- Create: `/Users/jonathan/lookit/render/fields.go`
- Modify: `/Users/jonathan/lookit/render/render.go`
- Update: `/Users/jonathan/lookit/render/testdata/basic.truecolor.golden` (via -update)

- [ ] **Step 1: Create `fields.go`**

Create `/Users/jonathan/lookit/render/fields.go`:

```go
package render

import "strings"

// fieldPrefixes are the line prefixes whose labels we color.
// We only style the prefix; the content after the colon is printed verbatim.
var fieldPrefixes = []string{
	"Login:",
	"Name:",
	"Plan:",
	"Project:",
	"Office:",
	"Office Phone:",
	"Home Phone:",
	"Directory:",
	"Shell:",
	"Last login",
	"No Plan.",
	"On since",
}

// highlightFields walks body line by line and re-emits each line. If a line
// begins with one of fieldPrefixes, the prefix is wrapped in theme.Field;
// the rest of the line is untouched.
func highlightFields(theme Theme, body []byte) string {
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
			sb.WriteString(line)
		}
	}
	return sb.String()
}
```

- [ ] **Step 2: Wire `highlightFields` into `Render`**

In `/Users/jonathan/lookit/render/render.go`, replace the body-emission section:

```go
	if len(body) == 0 && queryErr == nil {
		sb.WriteString(theme.Footer.Render("(no response body)"))
		sb.WriteString("\n")
	} else {
		sb.WriteString(highlightFields(theme, body))
		if len(body) > 0 && body[len(body)-1] != '\n' {
			sb.WriteString("\n")
		}
	}
```

- [ ] **Step 3: Run, observe golden mismatch**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/...
```

Expected: FAIL for `TestRender_BasicTrueColor` — output now includes cyan-bold labels, golden doesn't.

- [ ] **Step 4: Regenerate the truecolor golden, eyeball**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -run TestRender_BasicTrueColor -update -v
cat render/testdata/basic.truecolor.golden
```

Expected: golden now contains cyan-bold escapes around `Login:`, `Name:`, `Directory:`, `Shell:`, `Plan:`, `On since`. The content after each colon is plain.

- [ ] **Step 5: Confirm NoTTY golden is unchanged (field highlighting is a no-op for NoColor)**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/...
```

Expected: PASS for both `TestRender_BasicTrueColor` and `TestRender_BasicNoTTY`. The NoTTY golden didn't need an update because `highlightFields` returns `string(body)` unchanged when `theme.NoColor` is true.

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add render/fields.go render/render.go render/testdata/basic.truecolor.golden
git commit -m "feat(render): highlight Login:/Name:/Plan: etc. field labels"
```

---

## Task 12: `render` — ASCII art preservation

**Files:**
- Create: `/Users/jonathan/lookit/render/testdata/ascii-art.input`
- Create: `/Users/jonathan/lookit/render/testdata/ascii-art.truecolor.golden`
- Modify: `/Users/jonathan/lookit/render/render_test.go`

- [ ] **Step 1: Create the ASCII art input**

Create `/Users/jonathan/lookit/render/testdata/ascii-art.input`:

```
Login: bob
Plan:
   _______
  /       \
 |  o   o  |
 |    >    |
 |  \___/  |
  \_______/
   .plan ftw.
```

- [ ] **Step 2: Add the test**

Append to `/Users/jonathan/lookit/render/render_test.go`:

```go
func TestRender_AsciiArtPreserved(t *testing.T) {
	body := loadInput(t, "ascii-art")
	target := finger.Target{User: "bob", HostPort: "example.com:79", Raw: "bob@example.com"}
	meta := finger.Meta{Addr: "example.com:79", Elapsed: 90 * time.Millisecond, Bytes: len(body)}
	got := Render(target, body, meta, nil, colorprofile.TrueColor)
	compareGolden(t, "ascii-art", "truecolor", got)

	// Programmatic check: every input line that is not a field-prefixed line
	// must appear verbatim somewhere in the output.
	inputLines := strings.Split(string(body), "\n")
	for _, line := range inputLines {
		if line == "" || strings.HasPrefix(line, "Login:") || strings.HasPrefix(line, "Plan:") {
			continue
		}
		if !strings.Contains(got, line) {
			t.Errorf("ASCII art line not preserved verbatim:\n  line:   %q\n  output: %s", line, got)
		}
	}
}
```

- [ ] **Step 3: Run -update to create the golden**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -run TestRender_AsciiArtPreserved -update -v
cat render/testdata/ascii-art.truecolor.golden
```

Expected: golden contains the ASCII art with all spacing and slashes intact. Only `Login:` and `Plan:` are styled.

- [ ] **Step 4: Run all render tests**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/jonathan/lookit
git add render/testdata/ascii-art.input render/testdata/ascii-art.truecolor.golden render/render_test.go
git commit -m "test(render): preserve ASCII art verbatim through the renderer"
```

---

## Task 13: `render` — empty response, truncated response, error footer cases

**Files:**
- Create: `/Users/jonathan/lookit/render/testdata/empty.input` (empty file)
- Create: `/Users/jonathan/lookit/render/testdata/truncated.input`
- Create: `/Users/jonathan/lookit/render/testdata/timeout.input`
- Create their `.truecolor.golden` files via -update
- Modify: `/Users/jonathan/lookit/render/render_test.go`

- [ ] **Step 1: Create the input fixtures**

Create `/Users/jonathan/lookit/render/testdata/empty.input` as an empty file:

```bash
cd /Users/jonathan/lookit
: > render/testdata/empty.input
```

Create `/Users/jonathan/lookit/render/testdata/truncated.input`:

```
Login: alice
Name: Alice
Plan:
This is a body that the server kept streaming past 1 MiB. We only show
the part lookit captured; the footer notes the truncation.
```

Create `/Users/jonathan/lookit/render/testdata/timeout.input`:

```
Login: alice
Plan:
partial content before the read deadline fired
```

- [ ] **Step 2: Add the tests**

Append to `/Users/jonathan/lookit/render/render_test.go`:

```go
import "errors"

func TestRender_EmptyResponse(t *testing.T) {
	body := loadInput(t, "empty") // empty file
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 42 * time.Millisecond, Bytes: 0}
	got := Render(target, body, meta, nil, colorprofile.TrueColor)
	compareGolden(t, "empty", "truecolor", got)
}

func TestRender_Truncated(t *testing.T) {
	body := loadInput(t, "truncated")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 800 * time.Millisecond, Bytes: len(body), Truncated: true}
	got := Render(target, body, meta, nil, colorprofile.TrueColor)
	compareGolden(t, "truncated", "truecolor", got)
}

func TestRender_Timeout(t *testing.T) {
	body := loadInput(t, "timeout")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{Addr: "plan.cat:79", Elapsed: 30 * time.Second, Bytes: len(body), Truncated: true}
	got := Render(target, body, meta, errors.New("read response timed out after 30s"), colorprofile.TrueColor)
	compareGolden(t, "timeout", "truecolor", got)
}
```

(Merge `"errors"` into the existing import block.)

- [ ] **Step 3: Generate goldens with -update**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -update -v
```

Expected: all PASS, three new golden files written.

- [ ] **Step 4: Eyeball each golden**

Run:
```bash
cd /Users/jonathan/lookit
cat render/testdata/empty.truecolor.golden
cat render/testdata/truncated.truecolor.golden
cat render/testdata/timeout.truecolor.golden
```

Expected:
- empty: header + dim "(no response body)" + footer "0 B · 42ms".
- truncated: header + body + footer ending in "... · truncated".
- timeout: header + body + footer ending in "... · truncated" + a red error line below.

- [ ] **Step 5: Run without -update**

Run:
```bash
cd /Users/jonathan/lookit
go test ./render/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/jonathan/lookit
git add render/testdata/ render/render_test.go
git commit -m "test(render): empty, truncated, and timeout response cases"
```

---

## Task 14: `main.go` — wiring (happy path + arg parsing)

**Files:**
- Create: `/Users/jonathan/lookit/main.go`

- [ ] **Step 1: Implement main.go**

Create `/Users/jonathan/lookit/main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/charmbracelet/colorprofile"

	"github.com/jonathandeamer/lookit/finger"
	"github.com/jonathandeamer/lookit/render"
)

// Exit codes per sysexits.h-ish conventions.
const (
	exitOK       = 0
	exitNetwork  = 2
	exitUsage    = 64 // EX_USAGE
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Fprintln(os.Stderr, "usage: lookit user@host[:port]")
		fmt.Fprintln(os.Stderr, "       lookit @host[:port]")
		os.Exit(exitUsage)
	}

	target, err := finger.ParseTarget(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lookit: %v\n", err)
		os.Exit(exitUsage)
	}

	profile := colorprofile.Detect(os.Stdout, os.Environ())

	body, meta, queryErr := finger.Query(context.Background(), target)
	fmt.Print(render.Render(target, body, meta, queryErr, profile))

	if queryErr != nil {
		os.Exit(exitCodeFor(queryErr))
	}
}

// exitCodeFor maps Query errors to process exit codes. Network failures
// (refused, timeout, DNS) → 2; everything else → 2 as well for now.
func exitCodeFor(err error) int {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return exitNetwork
	}
	return exitNetwork
}
```

- [ ] **Step 2: Build the binary**

Run:
```bash
cd /Users/jonathan/lookit
go build -o lookit .
```

Expected: builds clean. No errors.

- [ ] **Step 3: Smoke-test against the example fingerverse**

Run (use a server you know is up — `graph.no` is famously reliable):
```bash
cd /Users/jonathan/lookit
./lookit london@graph.no
```

Expected: a meteogram, with `➜ london@graph.no` chrome at the top and `bytes · elapsed` at the bottom. ASCII weather glyphs intact.

If you don't have a working network or want to avoid hitting public servers during dev, fire up an in-process echoer:
```bash
cd /Users/jonathan/lookit
# in one terminal:
printf 'Login: alice\r\nName: Alice\r\nPlan:\r\nhello\r\n' | nc -l 7979
# in another terminal:
./lookit alice@127.0.0.1:7979
```

- [ ] **Step 4: Smoke-test `@host` form**

Run:
```bash
cd /Users/jonathan/lookit
./lookit @tilde.team
```

Expected: tilde.team banner + active user listing. If the network is down, use the local nc trick.

- [ ] **Step 5: Smoke-test error path (no such host)**

Run:
```bash
cd /Users/jonathan/lookit
./lookit alice@no-such-host.example
echo "exit code: $?"
```

Expected: header line with `➜ alice@no-such-host.example`, a red error line, exit code `2`.

- [ ] **Step 6: Smoke-test usage error**

Run:
```bash
cd /Users/jonathan/lookit
./lookit
echo "exit code: $?"
./lookit just-a-name
echo "exit code: $?"
```

Expected: usage line to stderr, exit code 64 both times.

- [ ] **Step 7: Smoke-test pipe (no styling)**

Run:
```bash
cd /Users/jonathan/lookit
./lookit london@graph.no | cat
```

Expected: no ANSI escape sequences when piped to `cat`. (The terminal-attached `cat` will still display them if you do `| cat -v`, but plain `cat` to your terminal will just pass them through. The real test: pipe to `od -c` and confirm no `\033` bytes.)

```bash
cd /Users/jonathan/lookit
./lookit london@graph.no | od -c | head -5
```

Expected: no `033` (escape) sequences in the output.

- [ ] **Step 8: Commit**

```bash
cd /Users/jonathan/lookit
git add main.go
git commit -m "feat(main): wire arg parse → finger → render → stdout"
```

---

## Task 15: GitHub Actions CI

**Files:**
- Create: `/Users/jonathan/lookit/.github/workflows/ci.yml`

- [ ] **Step 1: Create the workflow**

Run:
```bash
mkdir -p /Users/jonathan/lookit/.github/workflows
```

Create `/Users/jonathan/lookit/.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - name: vet
        run: go vet ./...
      - name: gofmt
        # gofmt -d prints diffs but always exits 0; we use -l (lists files
        # that would be reformatted) and fail if its output is non-empty.
        run: |
          out=$(gofmt -l .)
          if [ -n "$out" ]; then
            echo "gofmt would reformat the following files:"
            echo "$out"
            exit 1
          fi
      - name: test
        run: go test ./... -race
```

- [ ] **Step 2: Run all CI checks locally**

Run:
```bash
cd /Users/jonathan/lookit
go vet ./...
test -z "$(gofmt -l .)"
go test ./... -race
```

Expected: all succeed. If `gofmt -l` lists files, run `gofmt -w .` and re-commit those changes before pushing.

- [ ] **Step 3: Commit**

```bash
cd /Users/jonathan/lookit
git add .github/workflows/ci.yml
git commit -m "ci: vet, gofmt, race-tested unit tests on push and PR"
```

---

## Task 16: README — flesh out beyond the skeleton

**Files:**
- Modify: `/Users/jonathan/lookit/README.md`

- [ ] **Step 1: Rewrite the README**

Replace the contents of `/Users/jonathan/lookit/README.md` with:

```markdown
# lookit

A finger client for the modern terminal.

```
➜ alice@plan.cat   123ms ✦
Login: alice
Name: Alice Example
Directory: /home/alice
Shell: /bin/zsh
On since Mon Mar 10 09:14 (PST) on tty1
Plan:
This is my plan for today.
- finish lookit MVP
- have a snack

1.2 KiB · 123ms
```

Lookit talks [RFC 1288](https://www.rfc-editor.org/rfc/rfc1288) finger over TCP/79 and renders the response with chrome and structured field highlighting. Built with [Charm](https://charm.sh) tools.

## Install

```bash
go install github.com/jonathandeamer/lookit@latest
```

(Or clone and `go build .`)

## Usage

```bash
lookit alice@plan.cat        # finger a user
lookit @tilde.team           # finger a server (banner + user list)
lookit alice@example.com:79  # explicit port
```

Output styling adapts to your terminal's color capabilities. When stdout is piped or `NO_COLOR` is set, lookit emits plain text — `lookit user@host | grep` works as expected.

## What it doesn't do

- It doesn't post `.plan` files or write to finger servers. Read-only.
- It doesn't send `/W` (verbose). RFC 1288 §2.5.5 calls it out as privacy-sensitive.
- It doesn't run a daemon. There is no background polling.
- It doesn't follow the deprecated `user@host1@host2` forwarding form.

## Roadmap

This is Phase 1 (CLI MVP). Planned next:

- **Phase 2** — a TUI reader (Bubble Tea) for browsing.
- **Phase 3** — subscriptions (`lookit subscribe` + `lookit refresh` for watch-and-diff) and a curated catalog (`lookit discover`).
- **Phase 4** — polish: VHS demo gif, Homebrew tap.

Design spec: [`docs/superpowers/specs/2026-05-28-lookit-design.md`](docs/superpowers/specs/2026-05-28-lookit-design.md).

## License

TBD.
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jonathan/lookit
git add README.md
git commit -m "docs: README with example output, usage, and roadmap"
```

---

## Final verification

- [ ] **Step 1: Full local CI dry-run**

Run:
```bash
cd /Users/jonathan/lookit
go vet ./...
test -z "$(gofmt -l .)" && echo "gofmt clean"
go test ./... -race -v
go build -o lookit .
./lookit @tilde.team
```

Expected: everything green, binary produces a styled banner.

- [ ] **Step 2: Verify the git log reads well**

Run:
```bash
cd /Users/jonathan/lookit
git log --oneline
```

Expected: a clean linear history of 14ish small commits, each with a conventional-commits subject.

- [ ] **Step 3: Push (if a remote is configured) — user decision**

Skip if no GitHub remote has been set up yet. The user will create the repo on GitHub and push when they're ready.

---

## Notes for the implementer

- **Module path:** All imports use `github.com/jonathandeamer/lookit/...`. If the user changes this in Task 1, every import needs to track. A `gofmt`/`gopls`-aware editor will surface broken imports immediately.
- **Goldens have ANSI escapes:** Don't try to read them by eye. Trust the `-update` workflow and visually verify the *rendered* output by `cat`-ing the golden in a real terminal.
- **No real network in tests:** Every test in `finger/client_test.go` uses an in-process listener on `127.0.0.1:0`. CI never reaches out.
- **lipgloss + colorprofile interaction:** `colorprofile.Profile.Convert(c)` returns a `color.Color`. `lipgloss.Color(hex)` accepts hex strings, so we round-trip through `#RRGGBB`. This is verbose; if it gets uncomfortable, look at `lipgloss.NewStyle().Foreground(...)` with the new `lipgloss.Color` ANSI type wrappers — but the hex approach is portable across lipgloss versions.
