package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/charmbracelet/colorprofile"

	"github.com/jonathandeamer/lookit/finger"
	"github.com/jonathandeamer/lookit/render"
	"github.com/jonathandeamer/lookit/tui"
)

// Exit codes per sysexits.h-ish conventions.
const (
	exitOK      = 0
	exitNetwork = 2
	exitUsage   = 64 // EX_USAGE
)

var (
	version        = "dev"
	builtAt        = "unknown"
	runOneShotFunc = runOneShot
	startTUI       = func() error {
		profile := colorprofile.Detect(os.Stdout, os.Environ())
		return tui.Run(context.Background(), profile)
	}
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if err := startTUI(); err != nil {
			fmt.Fprintf(stderr, "lookit: %v\n", err)
			return exitNetwork
		}
		return exitOK
	}

	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		printUsage(stderr)
		return exitUsage
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, versionString())
		return exitOK
	}

	target, err := finger.ParseTarget(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "lookit: %v\n", err)
		return exitUsage
	}

	return runOneShotFunc(context.Background(), target, stdout)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  lookit")
	fmt.Fprintln(w, "  lookit user@host[:port]")
	fmt.Fprintln(w, "  lookit @host[:port]")
	fmt.Fprintln(w, "  lookit version")
}

func versionString() string {
	return fmt.Sprintf("lookit %s (built %s)", version, builtAt)
}

func runOneShot(ctx context.Context, target finger.Target, stdout io.Writer) int {
	profile := colorprofile.Detect(os.Stdout, os.Environ())
	body, meta, queryErr := finger.Query(ctx, target)
	fmt.Fprint(stdout, render.Render(target, body, meta, queryErr, profile))
	if queryErr != nil {
		return exitCodeFor(queryErr)
	}
	return exitOK
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
