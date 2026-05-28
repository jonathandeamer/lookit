package main

import (
	"errors"
	"net"
	"testing"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "dns error",
			err:  &net.DNSError{Err: "no such host", Name: "example.invalid"},
			want: exitNetwork,
		},
		{
			name: "generic error",
			err:  errors.New("read failed"),
			want: exitNetwork,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeFor(tc.err); got != tc.want {
				t.Fatalf("exitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
