package main

import (
	"bytes"
	"strings"
	"testing"
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
}

func TestResponseForUnknownUserIsExplicit(t *testing.T) {
	got := responseFor("carol")
	if !bytes.Contains(got, []byte("No such user")) {
		t.Fatalf("unknown user body %q, want an explicit miss", got)
	}
}
