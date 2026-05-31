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
