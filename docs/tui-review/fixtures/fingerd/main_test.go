package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jonathandeamer/lookit/finger"
)

func TestResponseForEmptyQueryServesList(t *testing.T) {
	got := responseFor("")
	if !bytes.Contains(got, []byte("Users currently online:")) {
		t.Fatalf("empty query body %q, want the list cue", got)
	}
	if !bytes.Contains(got, []byte("alice")) || !bytes.Contains(got, []byte("bob")) {
		t.Fatalf("empty query body %q, want alice and bob", got)
	}
}

func TestResponseForCRLFEmptyQueryMatchesBare(t *testing.T) {
	if !bytes.Equal(responseFor("\r\n"), responseFor("")) {
		t.Fatal("CRLF-terminated empty query should match the host listing")
	}
}

func TestResponseForAliceServesPlan(t *testing.T) {
	got := responseFor("alice")
	if !bytes.Contains(got, []byte("Login: alice")) {
		t.Fatalf("alice body %q, want Login: alice", got)
	}
	if !bytes.Contains(got, []byte("Alice Review")) {
		t.Fatalf("alice body %q, want the review-kit name", got)
	}
	if !strings.Contains(string(got), "Plan:") {
		t.Fatalf("alice body %q, want a Plan field", got)
	}
	if !bytes.Contains(got, []byte("finger://example.com/bob")) || !bytes.Contains(got, []byte("https://example.com/review")) {
		t.Fatalf("alice body %q, want a finger link and a URL for the links stills", got)
	}
}

func TestResponseForTruncHasNoTrailingNewline(t *testing.T) {
	got := responseFor("trunc")
	if len(got) == 0 || got[len(got)-1] == '\n' {
		t.Fatalf("trunc body %q, want a mid-line cut (no trailing newline)", got)
	}
	if !bytes.Contains(got, []byte("mid-senten")) {
		t.Fatalf("trunc body %q, want the cut plan text", got)
	}
}

func TestGenericBodyIsBareLogins(t *testing.T) {
	got := []byte(genericBody)
	if bytes.Contains(got, []byte("online")) {
		t.Fatal("generic body must not trip the named grid cue")
	}
	if !bytes.Contains(got, []byte("carol")) || !bytes.Contains(got, []byte("dave")) {
		t.Fatalf("generic body %q, want carol and dave", got)
	}
}

func TestResponseForUnknownUserIsExplicit(t *testing.T) {
	got := responseFor("eve")
	if !bytes.Contains(got, []byte("No such user")) {
		t.Fatalf("unknown user body %q, want an explicit miss", got)
	}
}

// The reader-scroll still waits on the line that sits at the top of the
// viewport after scrollStill scrolls; it has to exist, and the body has to
// outlast the tallest tape so the reader is scrollable at all.
func TestLongPlanScrollsAtEveryTapeHeight(t *testing.T) {
	const tallestTapeRows = 30

	got := responseFor("longplan")
	lines := strings.Split(strings.TrimSuffix(string(got), "\n"), "\n")
	if len(lines) <= tallestTapeRows {
		t.Fatalf("long plan is %d lines, want more than the tallest tape (%d rows)", len(lines), tallestTapeRows)
	}
	want := fmt.Sprintf("line %03d of", scrollStill+1)
	if !strings.Contains(string(got), want) {
		t.Fatalf("long plan body has no %q for the tape to wait on", want)
	}
	if scrollStill+tallestTapeRows > longPlanLines {
		t.Fatalf("scrolling %d lines can push line %d off the top of a %d-row viewport",
			scrollStill, scrollStill+1, tallestTapeRows)
	}
	if strings.Contains(string(got), "://") {
		t.Fatal("long plan must carry no links; tab-focus would change the still")
	}
}

func TestPingBothAcceptsTheFixture(t *testing.T) {
	addr := serveFixture(t, responseFor)
	generic := serveFixture(t, func(string) []byte { return []byte(genericBody) })

	if err := pingBoth(addr, generic); err != nil {
		t.Fatalf("pingBoth on the real fixture: %v", err)
	}
}

func TestPingBothRejectsAForeignListener(t *testing.T) {
	addr := serveFixture(t, responseFor)
	foreign := serveFixture(t, func(string) []byte { return []byte("some other daemon\n") })

	if err := pingBoth(addr, foreign); err == nil {
		t.Fatal("pingBoth = nil, want an error so a busy port stops the recording")
	}
}

func TestPingBothRejectsAClosedPort(t *testing.T) {
	if err := pingBoth("127.0.0.1:1", "127.0.0.1:1"); err == nil {
		t.Fatal("pingBoth = nil on a closed port, want a dial error")
	}
}

// serveFixture starts the daemon's own accept loop on an ephemeral loopback
// port and returns its address.
func serveFixture(t *testing.T, handler func(string) []byte) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveWith(conn, handler)
		}
	}()
	return ln.Addr().String()
}

func TestTruncQueryIsMarkedTruncated(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		serveWith(conn, responseFor)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addr := ln.Addr().String()
	body, meta, err := finger.Query(ctx, finger.Target{Query: "trunc", HostPort: addr, Raw: "trunc@" + addr})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !meta.Truncated {
		t.Fatalf("Truncated = false, body %q", body)
	}
	if !bytes.Contains(body, []byte("mid-senten")) {
		t.Fatalf("body %q, want the cut plan", body)
	}
}
