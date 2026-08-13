package main

import (
	"bytes"
	"context"
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
		serve(conn)
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
