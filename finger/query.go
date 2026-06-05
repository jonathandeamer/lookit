// Package finger implements an RFC 1288 finger client.
package finger

import (
	"errors"
	"fmt"
	"net"
	"strconv"
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
	if strings.Contains(hostport, "@") {
		return Target{}, errors.New("forwarded finger queries are not supported yet")
	}
	if hasControl(user) || hasControl(hostport) {
		return Target{}, errors.New("target contains control characters")
	}
	hostport, err := parseHostPort(hostport)
	if err != nil {
		return Target{}, err
	}
	return Target{User: user, HostPort: hostport, Raw: arg}, nil
}

// hasControl reports whether s contains an ASCII C0 control (< 0x20, including
// CR and LF) or DEL (0x7f). Such bytes in a query token would let a hostile or
// malformed target smuggle extra RFC 1288 query lines onto the wire.
func hasControl(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

func parseHostPort(s string) (string, error) {
	if strings.HasPrefix(s, "[") {
		close := strings.IndexByte(s, ']')
		if close < 0 {
			return "", errors.New("IPv6 literals must be bracketed, e.g. [::1]")
		}
		host := s[1:close]
		if host == "" {
			return "", errors.New("missing host after @")
		}
		suffix := s[close+1:]
		if suffix == "" {
			return net.JoinHostPort(host, "79"), nil
		}
		if !strings.HasPrefix(suffix, ":") {
			return "", fmt.Errorf("invalid host/port %q", s)
		}
		port, err := parsePort(suffix[1:])
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(host, port), nil
	}

	switch strings.Count(s, ":") {
	case 0:
		if s == "" {
			return "", errors.New("missing host after @")
		}
		return net.JoinHostPort(s, "79"), nil
	case 1:
		host, port, _ := strings.Cut(s, ":")
		if host == "" {
			return "", errors.New("missing host after @")
		}
		port, err := parsePort(port)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(host, port), nil
	default:
		return "", errors.New("IPv6 literals must be bracketed, e.g. [::1]")
	}
}

func parsePort(s string) (string, error) {
	if s == "" {
		return "", errors.New("invalid port")
	}
	port, err := strconv.ParseUint(s, 10, 16)
	if err != nil || port == 0 {
		return "", errors.New("invalid port")
	}
	return strconv.FormatUint(port, 10), nil
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
