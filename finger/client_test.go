package finger

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
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

	body, meta, err := Query(ctx, Target{Query: "alice", HostPort: fs.addr, Raw: "alice@" + fs.addr})
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

	body, _, err := Query(ctx, Target{Query: "", HostPort: fs.addr, Raw: "@" + fs.addr})
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

func TestQuery_BodyThenResetIsSuccess(t *testing.T) {
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
		_, _ = conn.Write([]byte("TELEHACK SYSTEM STATUS\r\noperator\r\n"))
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, meta, err := Query(ctx, Target{HostPort: ln.Addr().String()})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if meta.Truncated {
		t.Fatalf("Truncated = true, want false")
	}
	want := "TELEHACK SYSTEM STATUS\noperator\n"
	if string(body) != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}

func TestQuery_MidLineResetIsTruncated(t *testing.T) {
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
		// Body cut mid-line: no trailing newline before the reset.
		_, _ = conn.Write([]byte("Login     Name\r\nalice     Alice cut off here"))
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, meta, err := Query(ctx, Target{HostPort: ln.Addr().String()})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !meta.Truncated {
		t.Fatalf("Truncated = false, want true for a body cut mid-line by reset")
	}
	if len(body) == 0 {
		t.Fatalf("body is empty, want the partial body preserved")
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

func TestQuery_SizeCapAfterSanitize(t *testing.T) {
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
		buf := make([]byte, 64*1024)
		for i := range buf {
			buf[i] = 0xff
		}
		for written := 0; written < maxBodyBytes; written += len(buf) {
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
		t.Errorf("Truncated = false, want true after sanitize expansion exceeds cap")
	}
	if len(body) != maxBodyBytes {
		t.Errorf("len(body) = %d, want %d", len(body), maxBodyBytes)
	}
	if meta.Bytes != len(body) {
		t.Errorf("Bytes = %d, want %d", meta.Bytes, len(body))
	}
}

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
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Query took %v after cancel; want < 2s (cancel should close conn promptly)", elapsed)
	}
}

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

func TestQuery_DefangsInvalidUTF8Bytes(t *testing.T) {
	// Server emits a lone Latin-1 byte (0xE9, 'é') that is not valid UTF-8,
	// alongside a valid UTF-8 'ü' (0xC3 0xBC). Per RFC 1288 §3.3 the client
	// defangs the invalid byte to "\xe9" while leaving valid UTF-8 intact.
	fs := newFakeServer(t, func(line string) []byte {
		return []byte{'P', 'l', 'a', 'n', ':', '\r', '\n',
			'c', 'a', 'f', 0xE9, ' ', 0xC3, 0xBC, '\r', '\n'}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, _, err := Query(ctx, Target{HostPort: fs.addr})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := "Plan:\ncaf\\xe9 ü\n"
	if string(body) != want {
		t.Errorf("body:\n  got:  %q\n  want: %q", string(body), want)
	}
}

func TestQueryRejectsControlCharsInQuery(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	received := make(chan []byte, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			received <- nil
			return
		}
		defer c.Close()
		// Give the client a generous window to write anything it wants to.
		_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		buf, _ := io.ReadAll(c)
		received <- buf
	}()

	tgt := Target{Query: "a\r\nb", HostPort: ln.Addr().String()}
	_, _, queryErr := Query(context.Background(), tgt)

	if queryErr == nil {
		t.Fatal("Query with control char in query = nil error, want error")
	}
	if got := <-received; len(got) != 0 {
		t.Errorf("server received %d bytes %q; guard must fire before any write", len(got), got)
	}
}

func TestQuery_DefangsControlBytes(t *testing.T) {
	// Server sends a body containing an ESC sequence and a BEL, ending in CRLF.
	fs := newFakeServer(t, func(line string) []byte {
		return []byte("Name: \x1b[31mEvil\x1b[0m\x07\r\n")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	body, meta, err := Query(ctx, Target{HostPort: fs.addr})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	got := string(body)
	want := "Name: ^[[31mEvil^[[0m^G\n"
	if got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	// No live ESC may survive into the body handed to callers.
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("body still contains a raw ESC: %q", got)
	}
	// meta.Bytes reflects the post-sanitize length.
	if meta.Bytes != len(body) {
		t.Errorf("meta.Bytes = %d, want %d", meta.Bytes, len(body))
	}
	// Body ends in newline, so it must not be marked truncated.
	if meta.Truncated {
		t.Errorf("meta.Truncated = true, want false")
	}
}
