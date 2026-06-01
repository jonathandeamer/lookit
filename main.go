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
	detectProfile  = colorprofile.Detect
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
	outProfile := detectProfile(stdout, os.Environ())
	errProfile := detectProfile(stderr, os.Environ())

	if len(args) == 0 {
		if err := startTUI(); err != nil {
			fmt.Fprintln(stderr, render.ErrorLine(err.Error(), errProfile))
			return exitNetwork
		}
		return exitOK
	}

	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stderr, render.Usage(errProfile))
		return exitUsage
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, render.Version(versionString(), outProfile))
		return exitOK
	}

	target, err := finger.ParseTarget(args[0])
	if err != nil {
		fmt.Fprintln(stderr, render.ErrorLine(err.Error(), errProfile))
		return exitUsage
	}

	return runOneShotFunc(context.Background(), target, stdout)
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
