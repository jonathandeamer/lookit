package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/colorprofile"

	"github.com/jonathandeamer/lookit/render"
	"github.com/jonathandeamer/lookit/tui"
)

// Exit codes: lookit is a TUI-only finger browser, so there is no per-result
// network exit code. 0 is a clean session; 1 is any startup/usage failure.
const (
	exitOK    = 0
	exitError = 1
)

var (
	version       = "dev"
	builtAt       = "unknown"
	detectProfile = colorprofile.Detect
	startTUI      = func(opts tui.Options) error {
		profile := colorprofile.Detect(os.Stdout, os.Environ())
		return tui.Run(context.Background(), profile, opts)
	}
)

// init fills version/builtAt from the embedded build info when they were not set
// via -ldflags, so `go install …@latest` shows a real version + date instead of
// "dev"/"unknown". Release builds (ldflags set) keep their injected values.
func init() {
	if version != "dev" && builtAt != "unknown" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	if builtAt == "unknown" {
		for _, s := range info.Settings {
			if s.Key == "vcs.time" {
				builtAt = vcsDate(s.Value)
			}
		}
	}
}

// vcsDate trims an RFC 3339 VCS timestamp to its date portion.
func vcsDate(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable router. lookit always opens the TUI; a single positional
// argument seeds it. -h/--help and -v/--version are the only flags.
func run(args []string, stdout, stderr io.Writer) int {
	outProfile := detectProfile(stdout, os.Environ())
	errProfile := detectProfile(stderr, os.Environ())

	var positional []string
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stdout, render.Usage(outProfile))
			return exitOK
		case "-v", "--version":
			fmt.Fprintln(stdout, render.Version(versionString(), outProfile))
			return exitOK
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprint(stderr, render.Usage(errProfile))
				return exitError
			}
			positional = append(positional, a)
		}
	}

	if len(positional) > 1 {
		fmt.Fprint(stderr, render.Usage(errProfile))
		return exitError
	}

	// Seed is true whenever a positional arg was supplied, even a blank one
	// (lookit ""): the TUI replays it through submit() so a blank/malformed arg
	// shows its parse error in-place rather than silently landing.
	seed := len(positional) == 1
	query := ""
	if seed {
		query = positional[0]
	}

	if err := startTUI(tui.Options{InitialQuery: query, Seed: seed, Version: version, BuiltAt: builtAt}); err != nil {
		fmt.Fprintln(stderr, render.ErrorLine(err.Error(), errProfile))
		return exitError
	}
	return exitOK
}

func versionString() string {
	return fmt.Sprintf("lookit %s (built %s)", version, builtAt)
}
