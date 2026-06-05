package finger

import "testing"

func TestParseTarget(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    Target
		wantErr bool
	}{
		{
			name:  "user@host",
			input: "alice@plan.cat",
			want:  Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"},
		},
		{
			name:  "@host (server query)",
			input: "@tilde.team",
			want:  Target{User: "", HostPort: "tilde.team:79", Raw: "@tilde.team"},
		},
		{
			name:  "user@host:port",
			input: "alice@example.com:7979",
			want:  Target{User: "alice", HostPort: "example.com:7979", Raw: "alice@example.com:7979"},
		},
		{
			name:  "@host:port",
			input: "@example.com:7979",
			want:  Target{User: "", HostPort: "example.com:7979", Raw: "@example.com:7979"},
		},
		{
			name:  "finger:// scheme with path",
			input: "finger://via.sour.is/xuu",
			want:  Target{User: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
		},
		{
			name:  "path-style, no scheme",
			input: "via.sour.is/xuu",
			want:  Target{User: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
		},
		{
			name:  "path-style with explicit port",
			input: "via.sour.is:7979/xuu",
			want:  Target{User: "xuu", HostPort: "via.sour.is:7979", Raw: "xuu@via.sour.is:7979"},
		},
		{
			name:  "finger:// scheme, host only -> host query",
			input: "finger://plan.cat",
			want:  Target{User: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
		},
		{
			name:  "finger:// scheme with userinfo",
			input: "finger://user@host",
			want:  Target{User: "user", HostPort: "host:79", Raw: "user@host"},
		},
		{
			name:  "path-style, trailing slash, empty user -> host query",
			input: "plan.cat/",
			want:  Target{User: "", HostPort: "plan.cat:79", Raw: "@plan.cat"},
		},
		{
			name:  "mixed-case scheme is stripped",
			input: "FINGER://via.sour.is/xuu",
			want:  Target{User: "xuu", HostPort: "via.sour.is:79", Raw: "xuu@via.sour.is"},
		},
		{
			name:  "user with bracketed IPv6 defaults port",
			input: "alice@[::1]",
			want:  Target{User: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
		},
		{
			name:  "user with bracketed IPv6 explicit port",
			input: "alice@[::1]:7979",
			want:  Target{User: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
		},
		{
			name:  "host query with bracketed IPv6 defaults port",
			input: "@[::1]",
			want:  Target{User: "", HostPort: "[::1]:79", Raw: "@[::1]"},
		},
		{
			name:  "host query with bracketed IPv6 explicit port",
			input: "@[::1]:7979",
			want:  Target{User: "", HostPort: "[::1]:7979", Raw: "@[::1]:7979"},
		},
		{
			name:  "finger scheme with bracketed IPv6 path",
			input: "finger://[::1]/alice",
			want:  Target{User: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
		},
		{
			name:  "path-style bracketed IPv6 defaults port",
			input: "[::1]/alice",
			want:  Target{User: "alice", HostPort: "[::1]:79", Raw: "alice@[::1]"},
		},
		{
			name:  "path-style bracketed IPv6 explicit port",
			input: "[::1]:7979/alice",
			want:  Target{User: "alice", HostPort: "[::1]:7979", Raw: "alice@[::1]:7979"},
		},
		{
			name:    "missing @",
			input:   "alice",
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "@ with no host",
			input:   "alice@",
			wantErr: true,
		},
		{
			name:    "forwarded user query rejected for now",
			input:   "alice@plan.cat@tilde.team",
			wantErr: true,
		},
		{
			name:    "forwarded host query rejected for now",
			input:   "@plan.cat@tilde.team",
			wantErr: true,
		},
		{
			name:    "empty port",
			input:   "alice@example.com:",
			wantErr: true,
		},
		{
			name:    "non-numeric port",
			input:   "alice@example.com:abc",
			wantErr: true,
		},
		{
			name:    "out-of-range port",
			input:   "alice@example.com:99999",
			wantErr: true,
		},
		{
			name:    "zero port",
			input:   "alice@example.com:0",
			wantErr: true,
		},
		{
			name:    "unbracketed IPv6",
			input:   "alice@::1",
			wantErr: true,
		},
		{
			name:    "unclosed IPv6 bracket",
			input:   "alice@[::1",
			wantErr: true,
		},
		{
			name:    "bracketed IPv6 empty port",
			input:   "alice@[::1]:",
			wantErr: true,
		},
		{
			name:    "bracketed IPv6 non-numeric port",
			input:   "alice@[::1]:abc",
			wantErr: true,
		},
		{
			name:    "control char CR+LF in user",
			input:   "a\r\nb@host",
			wantErr: true,
		},
		{
			name:    "control char NUL in host",
			input:   "u@ho\x00st",
			wantErr: true,
		},
		{
			name:    "DEL in user",
			input:   "a\x7f@host",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTarget(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseTarget(%q): expected error, got nil", tc.input)
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
