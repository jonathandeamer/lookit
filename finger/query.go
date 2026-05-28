// Package finger implements an RFC 1288 finger client.
package finger

import (
	"errors"
	"strings"
)

// Target identifies a finger query: an optional user and a host:port pair.
type Target struct {
	User     string // empty for the bare "@host" form
	HostPort string // always "host:port"; port defaults to "79"
	Raw      string // original argument string, e.g. "alice@plan.cat"
}

// ParseTarget parses one of these forms:
//
//	user@host
//	@host
//	user@host:port
//	@host:port
//
// Returns an error for missing "@", empty input, or empty host.
func ParseTarget(arg string) (Target, error) {
	if arg == "" {
		return Target{}, errors.New("empty target")
	}
	at := strings.Index(arg, "@")
	if at < 0 {
		return Target{}, errors.New("target must be of the form user@host or @host")
	}
	user := arg[:at]
	hostport := arg[at+1:]
	if hostport == "" {
		return Target{}, errors.New("missing host after @")
	}
	if !strings.Contains(hostport, ":") {
		hostport = hostport + ":79"
	}
	return Target{User: user, HostPort: hostport, Raw: arg}, nil
}
