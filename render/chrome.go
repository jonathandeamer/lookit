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

func renderFooter(theme Theme, meta finger.Meta, notice string) string {
	stats := fmt.Sprintf("%s · %s", fmtBytes(meta.Bytes), fmtElapsed(meta.Elapsed))
	line := theme.Footer.Render(stats)
	if notice != "" {
		line += " " + theme.Footer.Render("·") + " " + theme.Warning.Render(notice)
	}
	return "\n" + line + "\n"
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

func fmtBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	}
}
