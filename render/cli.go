package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

// Usage returns lookit's CLI help block. On no-colour profiles
// (Ascii/NoTTY) the Theme styles are no-ops, so the output is byte-identical
// to the plain help text; on colour profiles headings, commands, syntax, and
// supporting notes use the existing adaptive theme.
func Usage(profile colorprofile.Profile) string {
	t := NewTheme(profile)
	cmd := t.Target.Render("lookit")
	var b strings.Builder
	fmt.Fprintln(&b, "A finger client built for exploring, not just querying.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Usage:"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("[TARGET]"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Targets:"))
	fmt.Fprintf(&b, "  %s    look up a person\n", t.Field.Render("user@host[:port]"))
	fmt.Fprintf(&b, "  %s        browse a host\n", t.Field.Render("@host[:port]"))
	fmt.Fprintf(&b, "  %s  open a finger URL\n", t.Field.Render("finger://host/user"))
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "  %s\n", t.Footer.Render("Ports default to 79. One-relay forwarding is also supported."))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Options:"))
	fmt.Fprintf(&b, "  %s       show help\n", t.Field.Render("-h, --help"))
	fmt.Fprintf(&b, "  %s    show version\n", t.Field.Render("-v, --version"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Examples:"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("jonathan@tilde.team"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("@plan.cat"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("Press ? inside lookit for keyboard shortcuts."))
	fmt.Fprintln(&b, t.Footer.Render("See man lookit for the full reference."))
	return b.String()
}

// Version styles a pre-formatted version line ("lookit <rest>"). On no-colour
// profiles it returns the line unchanged; otherwise it accents the leading
// "lookit" token and dims the remainder.
func Version(line string, profile colorprofile.Profile) string {
	t := NewTheme(profile)
	if t.NoColor {
		return line
	}
	name, rest, found := strings.Cut(line, " ")
	if !found {
		return t.Target.Render(line)
	}
	return t.Target.Render(name) + " " + t.Footer.Render(rest)
}

// ErrorLine returns "lookit: <msg>", in the error style on colour profiles and
// plain otherwise. Callers add the trailing newline.
func ErrorLine(msg string, profile colorprofile.Profile) string {
	t := NewTheme(profile)
	return t.ErrLine.Render("lookit: " + msg)
}

// InvocationError returns a specific CLI argument error followed by a short
// route to the full help. Unlike ErrorLine, it includes trailing newlines.
func InvocationError(message string, profile colorprofile.Profile) string {
	t := NewTheme(profile)
	var b strings.Builder
	fmt.Fprintln(&b, ErrorLine(message, profile))
	fmt.Fprintln(&b, t.Footer.Render("Try 'lookit --help' for usage."))
	return b.String()
}
