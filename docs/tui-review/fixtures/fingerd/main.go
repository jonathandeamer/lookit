// Command fingerd is a loopback RFC 1288 server for docs/tui-review tapes.
// It never faces the network: listen defaults to 127.0.0.1:2479 and the
// canned bodies are the only responses. Not part of the lookit product.
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

const defaultAddr = "127.0.0.1:2479"

const listBody = "Users currently online:\n\nalice bob\n"

const aliceBody = "Login: alice\nName: Alice Review\nPlan:\nA short .plan for the visual review kit.\n"

func main() {
	addr := flag.String("addr", defaultAddr, "listen address (loopback)")
	flag.Parse()
	if err := listenAndServe(*addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func listenAndServe(addr string) error {
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
		go serve(conn)
	}
}

func serve(conn net.Conn) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && err != io.EOF {
		return
	}
	_, _ = conn.Write(responseFor(line))
}

func responseFor(line string) []byte {
	query := strings.TrimSuffix(line, "\n")
	query = strings.TrimSuffix(query, "\r")
	query = strings.TrimSpace(query)
	switch query {
	case "", "@":
		return []byte(listBody)
	case "alice":
		return []byte(aliceBody)
	default:
		return []byte("Login: " + query + "\nNo such user.\n")
	}
}
