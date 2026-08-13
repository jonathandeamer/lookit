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

func main() {
	addr := flag.String("addr", defaultAddr, "listen address (loopback, named listing)")
	genericAddr := flag.String("generic-addr", defaultGenericAddr, "listen address for the generic listing")
	flag.Parse()

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

func serve(conn net.Conn) {
	serveWith(conn, responseFor)
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
	case "trunc":
		return []byte(truncBody)
	default:
		return []byte("Login: " + queryOf(line) + "\nNo such user.\n")
	}
}
