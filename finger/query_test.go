package finger

import (
	"errors"
	"testing"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    Target
		wantErr string
	}{
		{
			name:  "user@host",
			input: "alice@plan.cat",
			want:  Target{Query: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"},
		},
		{
			name:  "@host (server query)",
			input: "@tilde.team",
			want:  Target{Query: "", HostPort: "tilde.team:79", Raw: "@tilde.team"},
		},
		{
			name:  "user@host:port",
			input: "alice@example.com:7979",
			want:  Target{Query: "alice", HostPort: "example.com:7979", Raw: "alice@example.com:7979"},
		},
		{
			name:  "@host:port",
			input: "@example.com:7979",
			want:  Target{Query: "", HostPort: "example.com:7979", Raw: "@example.com:7979"},
		},
		{
			name:  "forwarded user query",
			input: "alice@tilde.team@thebackupbox.net",
			want:  Target{Query: "alice@tilde.team", HostPort: "thebackupbox.net:79", Raw: "alice@tilde.team@thebackupbox.net"},
		},
		{
			name:  "forwarded host query",
			input: "@tilde.team@thebackupbox.net",
			want:  Target{Query: "@tilde.team", HostPort: "thebackupbox.net:79", Raw: "@tilde.team@thebackupbox.net"},
		},
		{
			name:  "forwarded user query with relay port",
			input: "alice@tilde.team@thebackupbox.net:7979",
			want:  Target{Query: "alice@tilde.team", HostPort: "thebackupbox.net:7979", Raw: "alice@tilde.team@thebackupbox.net:7979"},
		},
		{
			name:  "forwarded host query with relay port",
			input: "@tilde.team@thebackupbox.net:7979",
			want:  Target{Query: "@tilde.team", HostPort: "thebackupbox.net:7979", Raw: "@tilde.team@thebackupbox.net:7979"},
		},
		{
			name:  "forwarded user query with inner bracketed IPv6 host",
			input: "alice@[::1]@thebackupbox.net",
			want:  Target{Query: "alice@[::1]", HostPort: "thebackupbox.net:79", Raw: "alice@[::1]@thebackupbox.net"},
		},
		{
			name:  "finger:// scheme with path",
			input: "finger://via.sour.is/xuu",
			want:  Target{Query: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
		},
		{
			name:  "path-style, no scheme",
			input: "via.sour.is/xuu",
			want:  Target{Query: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
		},
		{
			name:  "path-style with explicit port",
			input: "via.sour.is:7979/xuu",
			want:  Target{Query: "xuu", HostPort: "via.sour.is:7979", Raw: "xuu@via.sour.is:7979"},
		},
		{
			name:  "finger:// scheme, host only -> host query",
			input: "finger://plan.cat",
			want:  Target{Query: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
		},
		{
			name:  "finger:// scheme with userinfo",
			input: "finger://user@host",
			want:  Target{Query: "user", HostPort: "host:79", Raw: "user@host"},
		},
		{
			name:  "path-style, trailing slash, empty user -> host query",
			input: "plan.cat/",
			want:  Target{Query: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
		},
		{
			name:  "mixed-case scheme is stripped",
			input: "FINGER://via.sour.is/xuu",
			want:  Target{Query: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
		},
		{
			name:  "user with bracketed IPv6 defaults port",
			input: "alice@[::1]",
			want:  Target{Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
		},
		{
			name:  "user with bracketed IPv6 explicit port",
			input: "alice@[::1]:7979",
			want:  Target{Query: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
		},
		{
			name:  "host query with bracketed IPv6 defaults port",
			input: "@[::1]",
			want:  Target{Query: "", HostPort: "[::1]:79", Raw: "@[::1]"},
		},
		{
			name:  "host query with bracketed IPv6 explicit port",
			input: "@[::1]:7979",
			want:  Target{Query: "", HostPort: "[::1]:7979", Raw: "@[::1]:7979"},
		},
		{
			name:  "finger scheme with bracketed IPv6 path",
			input: "finger://[::1]/alice",
			want:  Target{Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
		},
		{
			name:  "path-style bracketed IPv6 defaults port",
			input: "[::1]/alice",
			want:  Target{Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
		},
		{
			name:  "path-style bracketed IPv6 explicit port",
			input: "[::1]:7979/alice",
			want:  Target{Query: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
		},
		{
			name:    "missing @",
			input:   "alice",
			wantErr: "target must be of the form user@host or @host",
		},
		{
			name:    "empty",
			input:   "",
			wantErr: "empty target",
		},
		{
			name:    "@ with no host",
			input:   "alice@",
			wantErr: "missing host after @",
		},
		{
			name:    "empty port",
			input:   "alice@example.com:",
			wantErr: "invalid port",
		},
		{
			name:    "non-numeric port",
			input:   "alice@example.com:abc",
			wantErr: "invalid port",
		},
		{
			name:    "out-of-range port",
			input:   "alice@example.com:99999",
			wantErr: "invalid port",
		},
		{
			name:    "zero port",
			input:   "alice@example.com:0",
			wantErr: "invalid port",
		},
		{
			name:    "unbracketed IPv6",
			input:   "alice@::1",
			wantErr: "IPv6 literals must be bracketed, e.g. [::1]",
		},
		{
			name:    "unclosed IPv6 bracket",
			input:   "alice@[::1",
			wantErr: "IPv6 literals must be bracketed, e.g. [::1]",
		},
		{
			name:    "bracketed IPv6 empty port",
			input:   "alice@[::1]:",
			wantErr: "invalid port",
		},
		{
			name:    "bracketed IPv6 non-numeric port",
			input:   "alice@[::1]:abc",
			wantErr: "invalid port",
		},
		{
			name:    "control char CR+LF in user",
			input:   "a\r\nb@host",
			wantErr: "target contains control characters",
		},
		{
			name:    "control char NUL in host",
			input:   "u@ho\x00st",
			wantErr: "target contains control characters",
		},
		{
			name:    "DEL in user",
			input:   "a\x7f@host",
			wantErr: "target contains control characters",
		},
		{
			name:    "multiple forwarding relays rejected",
			input:   "alice@h1@h2@relay",
			wantErr: "forwarding through multiple relays is not supported yet",
		},
		{
			name:    "multiple forwarding relays rejected for host query",
			input:   "@h1@h2@relay",
			wantErr: "forwarding through multiple relays is not supported yet",
		},
		{
			name:    "inner forwarded user port rejected",
			input:   "alice@tilde.team:7979@thebackupbox.net",
			wantErr: "forwarded host ports are not supported; put a port only on the relay",
		},
		{
			name:    "inner forwarded host port rejected",
			input:   "@tilde.team:7979@thebackupbox.net",
			wantErr: "forwarded host ports are not supported; put a port only on the relay",
		},
		{
			name:    "URL forwarding rejected with deferred message",
			input:   "finger://thebackupbox.net/alice@tilde.team",
			wantErr: "forwarding in finger:// URLs is not supported yet; use user@host@relay",
		},
		{
			name:    "URL userinfo forwarding rejected with deferred message",
			input:   "finger://alice@tilde.team@thebackupbox.net",
			wantErr: "forwarding in finger:// URLs is not supported yet; use user@host@relay",
		},
		{
			name:    "malformed forwarding missing inner host",
			input:   "alice@@thebackupbox.net",
			wantErr: "forwarded targets must be user@host@relay or @host@relay",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTarget(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseTarget(%q): expected error %q, got nil", tc.input, tc.wantErr)
				}
				if got := err.Error(); got != tc.wantErr {
					t.Fatalf("ParseTarget(%q) error = %q, want %q", tc.input, got, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTarget(%q): unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseTarget(%q):\n  got:  %#v\n  want: %#v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseTargetPinned(t *testing.T) {
	cases := []struct {
		name                 string
		input                string
		want                 Target
		wantErr              string
		wantServerForwarding bool
	}{
		{
			name:  "hostile port pinned to 79 and surfaced in Raw",
			input: "evil@example.com:22",
			want:  Target{Query: "evil", HostPort: "example.com:79", Raw: "evil@example.com:79"},
		},
		{
			// The regression: an out-of-range/garbage port would block the drill
			// under strict ParseTarget; here it is discarded, not rejected.
			name:  "out-of-range port discarded, not rejected",
			input: "alice@example.com:99999",
			want:  Target{Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
		},
		{
			name:  "zero port discarded, not rejected",
			input: "alice@example.com:0",
			want:  Target{Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
		},
		{
			name:  "no explicit port keeps clean Raw",
			input: "yalla@tilde.team",
			want:  Target{Query: "yalla", HostPort: "tilde.team:79", Raw: "yalla@tilde.team"},
		},
		{
			name:  "explicit :79 keeps clean Raw",
			input: "alice@example.com:79",
			want:  Target{Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
		},
		{
			name:  "bracketed IPv6 port pinned",
			input: "alice@[::1]:2222",
			want:  Target{Query: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]:79"},
		},
		{
			name:  "finger scheme link with hostile port pinned",
			input: "finger://example.com:31337/alice",
			want:  Target{Query: "alice", HostPort: "example.com:79", Raw: "alice@example.com:79"},
		},
		{
			// Host structure is still validated even though the port is not.
			name:    "unbracketed IPv6 still rejected",
			input:   "alice@fe80::1",
			wantErr: "IPv6 literals must be bracketed, e.g. [::1]",
		},
		{
			name:    "control char still rejected",
			input:   "a\r\nb@host",
			wantErr: "target contains control characters",
		},
		{
			name:                 "forwarded query still rejected",
			input:                "alice@plan.cat@tilde.team",
			wantErr:              ErrServerForwarding.Error(),
			wantServerForwarding: true,
		},
		{
			name:                 "forwarded host query still rejected",
			input:                "@plan.cat@tilde.team",
			wantErr:              ErrServerForwarding.Error(),
			wantServerForwarding: true,
		},
		{
			name:                 "multiple forwarded relays still rejected",
			input:                "alice@h1@h2@relay",
			wantErr:              ErrServerForwarding.Error(),
			wantServerForwarding: true,
		},
		{
			name:                 "forwarded URL query still rejected",
			input:                "finger://thebackupbox.net/alice@tilde.team",
			wantErr:              ErrServerForwarding.Error(),
			wantServerForwarding: true,
		},
		{
			name:                 "forwarded URL userinfo query still rejected",
			input:                "finger://alice@tilde.team@thebackupbox.net",
			wantErr:              ErrServerForwarding.Error(),
			wantServerForwarding: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTargetPinned(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseTargetPinned(%q): expected error %q, got nil", tc.input, tc.wantErr)
				}
				if got := err.Error(); got != tc.wantErr {
					t.Fatalf("ParseTargetPinned(%q) error = %q, want %q", tc.input, got, tc.wantErr)
				}
				if tc.wantServerForwarding && !errors.Is(err, ErrServerForwarding) {
					t.Fatalf("ParseTargetPinned(%q) error = %v, want errors.Is ErrServerForwarding", tc.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTargetPinned(%q): unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseTargetPinned(%q):\n  got:  %#v\n  want: %#v", tc.input, got, tc.want)
			}
		})
	}
}
