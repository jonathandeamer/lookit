package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/jonathandeamer/lookit/finger"
)

func renderHeader(theme Theme, t finger.Target, meta finger.Meta, success bool) string {
	parts := []string{
		theme.Arrow.Render("➜"),
		theme.Target.Render(t.Raw),
		theme.Latency.Render(fmtElapsed(meta.Elapsed)),
	}
	if success {
		parts = append(parts, theme.Sparkle.Render("✦"))
	}
	return strings.Join(parts, " ") + "\n"
}

func fmtElapsed(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

