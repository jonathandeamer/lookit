package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

// Usage returns lookit's CLI usage block. On no-colour profiles
// (Ascii/NoTTY) the Theme styles are no-ops, so the output is byte-identical
// to the plain usage text; on colour profiles the command and example tokens
// are accented.
func Usage(profile colorprofile.Profile) string {
	t := NewTheme(profile)
	cmd := t.Target.Render("lookit")
	var b strings.Builder
	fmt.Fprintln(&b, t.Footer.Render("usage:"))
	fmt.Fprintf(&b, "  %s\n", cmd)
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("user@host[:port]"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Field.Render("@host[:port]"))
	fmt.Fprintf(&b, "  %s %s\n", cmd, t.Footer.Render("--version"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, t.Footer.Render("press ? in lookit for keys"))
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
