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
