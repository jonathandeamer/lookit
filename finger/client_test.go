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
