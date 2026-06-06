// Package finger implements an RFC 1288 finger client.
package finger

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Target identifies a finger query and the host:port endpoint that receives it.
type Target struct {
	User     string // deprecated compatibility alias for Query
	Query    string // exact Finger query line without trailing CRLF
	HostPort string // always "host:port"; port defaults to "79"
	Raw      string // normalized argument string, e.g. "alice@plan.cat"
}

// QueryLine returns the exact line Query should send, without the trailing CRLF.
// The User fallback keeps manually-constructed legacy test targets working while
// call sites migrate to Query.
func (t Target) QueryLine() string {
	if t.Query != "" || t.User == "" {
		return t.Query
	}
	return t.User
}

// HostQuery reports whether a response can be treated as a host/list response.
// RFC 1288 forwarding permits a query like "@host@relay"; after parsing, that
// has Query "@host" and should route like a host query if the body is parseable.
func (t Target) HostQuery() bool {
	q := t.QueryLine()
	return q == "" || strings.HasPrefix(q, "@")
}

var (
	errMissingHost         = errors.New("missing host after @")
	errBracketIPv6         = errors.New("IPv6 literals must be bracketed, e.g. [::1]")
	errMultipleRelays      = errors.New("forwarding through multiple relays is not supported yet")
	errForwardedHostPort   = errors.New("forwarded host ports are not supported; put a port only on the relay")
	errURLForwarding       = errors.New("forwarding in finger:// URLs is not supported yet; use user@host@relay")
	errMalformedForwarding = errors.New("forwarded targets must be user@host@relay or @host@relay")
	ErrServerForwarding    = errors.New("forwarded targets from server responses are not opened")
)

// ParseTarget parses one of these forms:
//
//	user@host
//	@host
//	user@host:port
//	@host:port
//	user@host@relay
//	@host@relay
//	user@host@relay:port
//	@host@relay:port
//
// It also accepts a leading "finger://" scheme and path-style "host[:port]/user"
// addresses (e.g. "finger://via.sour.is/xuu" or "via.sour.is/xuu"), normalizing
// them into direct forms above. Forwarding in "finger://" URLs is deliberately
// rejected for now; use the explicit user@host@relay or @host@relay form.
// Returns an error for empty input or input with no "@", no "/", and no scheme,
// or for an empty host.
func ParseTarget(arg string) (Target, error) {
	return parseTarget(arg, false)
}

// ParseTargetPinned parses arg like ParseTarget but forces the port to finger's
// well-known 79, ignoring (and discarding) any explicit port in arg. It is used
// for targets lifted from a server's own response — finger:// links or
// "finger user@host" commands in a profile — which lookit must not let steer it
// at an arbitrary service (e.g. host:22). Pinning here makes that guarantee by
// construction, and because the port is discarded a malformed or out-of-range
// port in the response does not block the drill. The host is preserved (the
// Finger Ring is legitimately cross-host). Raw keeps the typed form unless the
// port was overridden, in which case it shows the pinned ":79" so the change is
// visible.
func ParseTargetPinned(arg string) (Target, error) {
	return parseTarget(arg, true)
}

func parseTarget(arg string, pin bool) (Target, error) {
	if arg == "" {
		return Target{}, errors.New("empty target")
	}
	if isDeferredURLForwarding(arg) {
		if pin {
			return Target{}, ErrServerForwarding
		}
		return Target{}, errURLForwarding
	}
	arg = normalizeTarget(arg)
	if hasControl(arg) {
		return Target{}, errors.New("target contains control characters")
	}

	switch strings.Count(arg, "@") {
	case 0:
		return Target{}, errors.New("target must be of the form user@host or @host")
	case 1:
		return parseDirectTarget(arg, pin)
	case 2:
		if pin {
			return Target{}, ErrServerForwarding
		}
		return parseForwardedTarget(arg)
	default:
		if pin {
			return Target{}, ErrServerForwarding
		}
		return Target{}, errMultipleRelays
	}
}

func parseDirectTarget(arg string, pin bool) (Target, error) {
	user, hostport, _ := strings.Cut(arg, "@")
	if hostport == "" {
		return Target{}, errMissingHost
	}
	host, rawPort, hasPort, err := splitHostPort(hostport)
	if err != nil {
		return Target{}, err
	}

	port := "79"
	if !pin && hasPort {
		if port, err = parsePort(rawPort); err != nil {
			return Target{}, err
		}
	}
	canonical := net.JoinHostPort(host, port)
	raw := arg
	if pin && hasPort && rawPort != "79" {
		raw = user + "@" + canonical // surface the overridden port as :79
	}
	return Target{User: user, Query: user, HostPort: canonical, Raw: raw}, nil
}

func parseForwardedTarget(arg string) (Target, error) {
	last := strings.LastIndex(arg, "@")
	if last <= 0 || last == len(arg)-1 {
		return Target{}, errMalformedForwarding
	}
	query := arg[:last]
	relay := arg[last+1:]

	if err := validateForwardQuery(query); err != nil {
		return Target{}, err
	}
	host, rawPort, hasPort, err := splitHostPort(relay)
	if err != nil {
		return Target{}, err
	}
	port := "79"
	if hasPort {
		if port, err = parsePort(rawPort); err != nil {
			return Target{}, err
		}
	}
	return Target{User: query, Query: query, HostPort: net.JoinHostPort(host, port), Raw: arg}, nil
}

func validateForwardQuery(query string) error {
	user, host, ok := strings.Cut(query, "@")
	if !ok || host == "" {
		return errMalformedForwarding
	}
	if user == "" && !strings.HasPrefix(query, "@") {
		return errMalformedForwarding
	}
	_, _, hasPort, err := splitHostPort(host)
	if err != nil {
		return err
	}
	if hasPort {
		return errForwardedHostPort
	}
	return nil
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

// splitHostPort splits a host token — "host", "host:port", "[ipv6]", or
// "[ipv6]:port" — into the host (IPv6 brackets stripped) and the raw port
// string. hasPort distinguishes "no port given" (host) from "an explicit but
// possibly empty port" (host:), so callers can default the former while still
// validating the latter. It checks bracket structure and colon placement but
// not the port value; callers decide whether to parse, default, or pin it.
func splitHostPort(s string) (host, port string, hasPort bool, err error) {
	if strings.HasPrefix(s, "[") {
		rb := strings.IndexByte(s, ']')
		if rb < 0 {
			return "", "", false, errBracketIPv6
		}
		host = s[1:rb]
		if host == "" {
			return "", "", false, errMissingHost
		}
		suffix := s[rb+1:]
		if suffix == "" {
			return host, "", false, nil
		}
		if !strings.HasPrefix(suffix, ":") {
			return "", "", false, fmt.Errorf("invalid host/port %q", s)
		}
		return host, suffix[1:], true, nil
	}

	switch strings.Count(s, ":") {
	case 0:
		if s == "" {
			return "", "", false, errMissingHost
		}
		return s, "", false, nil
	case 1:
		host, port, _ = strings.Cut(s, ":")
		if host == "" {
			return "", "", false, errMissingHost
		}
		return host, port, true, nil
	default:
		return "", "", false, errBracketIPv6
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

func isDeferredURLForwarding(arg string) bool {
	i := strings.Index(arg, "://")
	if i < 0 || !strings.EqualFold(arg[:i], "finger") {
		return false
	}
	rest := arg[i+len("://"):]
	if strings.Count(rest, "@") > 1 {
		return true
	}
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return false
	}
	return strings.Contains(rest[slash+1:], "@")
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
