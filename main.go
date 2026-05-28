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
	exitOK      = 0
	exitNetwork = 2
	exitUsage   = 64 // EX_USAGE
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
	os.Exit(exitOK)
}

// exitCodeFor maps Query errors to process exit codes. Network failures
// (refused, timeout, DNS) return 2; everything else returns 2 as well for now.
func exitCodeFor(err error) int {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return exitNetwork
	}
	return exitNetwork
}
