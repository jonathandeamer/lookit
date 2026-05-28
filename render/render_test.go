package render

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/jonathandeamer/lookit/finger"
)

var update = flag.Bool("update", false, "update golden files")

// loadInput reads testdata/<name>.input.
func loadInput(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".input"))
	if err != nil {
		t.Fatalf("read input %s: %v", name, err)
	}
	return b
}

// compareGolden compares got against testdata/<name>.<profile>.golden.
// With -update, writes got to the golden file.
func compareGolden(t *testing.T, name, profile, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+"."+profile+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated golden %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if got != string(want) {
		t.Errorf("output differs from golden %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}

func TestRender_BasicTrueColor(t *testing.T) {
	body := loadInput(t, "basic")
	target := finger.Target{User: "alice", HostPort: "plan.cat:79", Raw: "alice@plan.cat"}
	meta := finger.Meta{
		Addr:    "plan.cat:79",
		Elapsed: 123 * time.Millisecond,
		Bytes:   len(body),
	}
	got := Render(target, body, meta, nil, colorprofile.TrueColor)
	compareGolden(t, "basic", "truecolor", got)
}
