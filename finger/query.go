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
// It also accepts a leading "finger://" scheme and path-style "host[:port]/user"
// addresses (e.g. "finger://via.sour.is/xuu" or "via.sour.is/xuu"), normalizing
// them into the forms above. Returns an error for empty input or input with no
// "@", no "/", and no scheme, or for an empty host.
func ParseTarget(arg string) (Target, error) {
	if arg == "" {
		return Target{}, errors.New("empty target")
	}
	arg = normalizeTarget(arg)
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

// normalizeTarget rewrites scheme-prefixed and path-style addresses into the
// canonical "user@host" / "@host" forms ParseTarget understands. Plain
// "@"-forms pass through untouched. A finger user is a single token, so only
// the first "/" separates host from user.
func normalizeTarget(arg string) string {
	hadScheme := false
	if i := strings.Index(arg, "://"); i >= 0 && strings.EqualFold(arg[:i], "finger") {
		arg = arg[i+len("://"):]
		hadScheme = true
	}

	if strings.Contains(arg, "@") {
		return strings.TrimRight(arg, "/") // already canonical (covers finger://user@host)
	}
	if slash := strings.Index(arg, "/"); slash >= 0 {
		host := arg[:slash]
		user := strings.TrimRight(arg[slash+1:], "/")
		return user + "@" + host // user may be "" -> "@host"
	}
	arg = strings.TrimRight(arg, "/")
	if hadScheme {
		return "@" + arg // finger://host with no path is a bare host query
	}
	return arg // bare token, no @/scheme/slash: let ParseTarget reject it
}
