package finger

import "testing"

func TestParseTarget(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		want     Target
		wantErr  bool
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
