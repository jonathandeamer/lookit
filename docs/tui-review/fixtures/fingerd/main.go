// Command fingerd is a loopback RFC 1288 server for docs/tui-review tapes.
// It never faces the network: listen defaults to 127.0.0.1:2479 (named
// listing, alice, trunc) and 127.0.0.1:2480 (generic listing). Canned
// bodies are the only responses. Not part of the lookit product.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultAddr        = "127.0.0.1:2479"
	defaultGenericAddr = "127.0.0.1:2480"
)

const listBody = "Users currently online:\n\nalice bob\n"

const aliceBody = "Login: alice\nName: Alice Review\nPlan:\nA short .plan for the visual review kit.\nSee also finger://example.com/bob and https://example.com/review\n"

// truncBody is cut mid-line on purpose so a connection reset marks Meta.Truncated.
const truncBody = "Login: trunc\nName: Trunc Review\nPlan:\nThis line is cut mid-senten"

// Two spaced columns, no "online" cue, so ParseUsers takes the generic path.
const genericBody = "carol  Carol Review\ndave  Dave Review\n"

// longPlanLines is longer than the tallest tape (30 rows) so the reader is
// scrollable and the status bar carries a scroll percentage. Lines are
// numbered so a tape can Wait on a line that is on screen at every height:
// after scrolling down scrollStill lines, line scrollStill+1 is the top row
// whether the viewport is 20 or 30 rows tall.
const longPlanLines = 60

// scrollStill is how far docs/tui-review/responses-tour.tape scrolls before
// taking reader-scroll.png. Keep it under longPlanLines minus the tallest
// viewport so the wait line cannot scroll off the top.
const scrollStill = 12

const wrapContinuationMarker = "WRAP-CONTINUATION-MARKER"
const wrapLongURL = "https://example.com/this-is-one-deliberately-indivisible-hyphenated-address-that-must-stay-intact"

func longPlanBody() []byte {
	var b strings.Builder
	b.WriteString("Login: longplan\nName: Long Review\nPlan:\n")
	for i := 1; i <= longPlanLines; i++ {
		fmt.Fprintf(&b, "line %03d of a plan long enough to scroll\n", i)
	}
	return []byte(b.String())
}

func wrapPlanBody() []byte {
	stamp := "2026-08-20 14:32 — This timestamped note runs past eighty cells while remaining a single ordinary prose line."
	paragraph := "This representative paragraph stays on one physical line so review can show optional wrapping without inventing metadata; " + wrapContinuationMarker + " follows."
	dispatchUnit := "Archived dispatches sometimes arrive serialised into one physical line, with sentence after sentence preserving meaning but not a comfortable reading width. "
	extreme := strings.Repeat(dispatchUnit, 4)
	preformatted := []string{
		"    HOST             STATE       NOTE",
		"    relay.example    ready       aligned",
	}

	lines := []string{
		"Login: wrapplan",
		"Name: Wrap Review",
		"Plan:",
		stamp,
		"",
		paragraph,
		"",
	}
	lines = append(lines, preformatted...)
	lines = append(lines,
		"",
		"Read "+wrapLongURL+" when horizontal scrolling is useful.",
		"",
		extreme,
	)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func main() {
	addr := flag.String("addr", defaultAddr, "listen address (loopback, named listing)")
	genericAddr := flag.String("generic-addr", defaultGenericAddr, "listen address for the generic listing")
	ping := flag.Bool("ping", false, "check that both ports serve the fixture bodies, then exit")
	flag.Parse()

	if *ping {
		if err := pingBoth(*addr, *genericAddr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	errc := make(chan error, 2)
	go func() {
		errc <- listenAndServe(*genericAddr, func(string) []byte { return []byte(genericBody) })
	}()
	go func() {
		errc <- listenAndServe(*addr, responseFor)
	}()
	fmt.Fprintln(os.Stderr, <-errc)
	os.Exit(1)
}

// pingBoth checks that both ports answer an empty query with the fixture
// bodies. `make review-tui` polls this instead of sleeping a fixed interval:
// a port bound by something else fails the identity check, so a busy port
// stops the run instead of filling the frames with whatever that other
// process serves.
func pingBoth(addr, genericAddr string) error {
	if err := pingAddr(addr, listBody); err != nil {
		return fmt.Errorf("named listing: %w", err)
	}
	if err := pingAddr(genericAddr, genericBody); err != nil {
		return fmt.Errorf("generic listing: %w", err)
	}
	return nil
}

func pingAddr(addr, want string) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(conn, "\r\n"); err != nil {
		return err
	}
	got, err := io.ReadAll(conn)
	if err != nil {
		return err
	}
	if string(got) != want {
		return fmt.Errorf("%s answered %q, not the fixture body", addr, got)
	}
	return nil
}

func listenAndServe(addr string, handler func(string) []byte) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go serveWith(conn, handler)
	}
}

func serveWith(conn net.Conn, handler func(string) []byte) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && err != io.EOF {
		return
	}
	query := queryOf(line)
	_, _ = conn.Write(handler(line))
	if query == "trunc" {
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0)
		}
	}
}

func queryOf(line string) string {
	query := strings.TrimSuffix(line, "\n")
	query = strings.TrimSuffix(query, "\r")
	return strings.TrimSpace(query)
}

func responseFor(line string) []byte {
	switch queryOf(line) {
	case "", "@":
		return []byte(listBody)
	case "alice":
		return []byte(aliceBody)
	case "longplan":
		return longPlanBody()
	case "wrapplan":
		return wrapPlanBody()
	case "trunc":
		return []byte(truncBody)
	default:
		return []byte("Login: " + queryOf(line) + "\nNo such user.\n")
	}
}
