package render

import (
	"net"
	"strings"

	"github.com/jonathandeamer/lookit/finger"
)

// tildeTeamHost is the host whose finger daemon emits the server-specific
// "Pronouns:" field. Its Pronouns handling is gated to this host because
// Pronouns is a tilde.team convention, not a finger standard.
const tildeTeamHost = "tilde.team"

// isTildeTeam reports whether the target points at tilde.team (any port).
func isTildeTeam(t finger.Target) bool {
	host := t.HostPort
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.EqualFold(host, tildeTeamHost)
}

// extraFieldPrefixes returns host-specific label prefixes to highlight in
// addition to the generic finger fields. Only tilde.team contributes one
// ("Pronouns:"); every other host returns nil.
func extraFieldPrefixes(t finger.Target) []string {
	if isTildeTeam(t) {
		return []string{"Pronouns:"}
	}
	return nil
}

// pronounsInlinePrefix is the inline form emitted by tilde.team's daemon:
// "Pronouns:" followed by a single space and the value.
const pronounsInlinePrefix = "Pronouns: "

// reflowPronouns rewrites an inline "Pronouns: <value>" line into a block that
// matches the Plan:/Project: layout — the label on its own line and the value
// on the next line, indented two spaces. A bare "Pronouns:" (no value) and all
// other lines pass through unchanged. The body's line structure is otherwise
// preserved.
func reflowPronouns(body []byte) []byte {
	lines := strings.SplitAfter(string(body), "\n")
	var sb strings.Builder
	for _, line := range lines {
		// Split the trailing newline (if any) off so we reason about content.
		content := line
		nl := ""
		if strings.HasSuffix(content, "\n") {
			content = content[:len(content)-1]
			nl = "\n"
		}
		if strings.HasPrefix(content, pronounsInlinePrefix) {
			value := content[len(pronounsInlinePrefix):]
			if value != "" {
				sb.WriteString("Pronouns:\n  ")
				sb.WriteString(value)
				sb.WriteString(nl)
				continue
			}
		}
		sb.WriteString(line)
	}
	return []byte(sb.String())
}
