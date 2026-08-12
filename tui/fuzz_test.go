package tui

import "testing"

// FuzzParseUsers asserts the host-listing parser never panics on arbitrary
// bytes and never emits a wholly-empty entry. ParseUsers runs on untrusted
// server responses, so a panic or a hang (caught by the fuzzer's timeout) is a
// denial-of-service, and an empty entry would be a selectable list row that
// points nowhere.
func FuzzParseUsers(f *testing.F) {
	f.Add([]byte("users currently logged in are:\n\nalrs\tdtracker\tkapad\n"))
	f.Add([]byte("Login    Name\nalice    Alice Example\nbob      Bob\n"))
	f.Add([]byte("just a plain profile\nPlan: hello\n"))
	f.Add([]byte(""))
	f.Add([]byte("\x00\x00 garbage \xff\xfe"))

	f.Fuzz(func(t *testing.T, body []byte) {
		users, _ := ParseUsers(body, "")
		for _, u := range users {
			if u.Login == "" && u.Target == "" {
				t.Fatalf("ParseUsers returned an empty entry for %q", body)
			}
		}
	})
}
