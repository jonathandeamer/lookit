package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestStatusBarProfileShowsBreadcrumb(t *testing.T) {
	b := statusBar{host: "@tilde.team", user: "jonathan", escTarget: "@tilde.team",
		meta: "1.2 KB", hints: "esc back · ? help", width: 80, styles: newStyles(true)}
	out := b.render()
	for _, want := range []string{"@tilde.team", "jonathan", "◂ esc: @tilde.team", "1.2 KB", "? help"} {
		if !strings.Contains(out, want) {
			t.Fatalf("bar %q missing %q", out, want)
		}
	}
	if w := lipgloss.Width(out); w != 80 {
		t.Fatalf("bar width = %d, want 80", w)
	}
}

func TestStatusBarDirectoryHasNoUserHalf(t *testing.T) {
	b := statusBar{host: "@tilde.team", meta: "3 users",
		hints: "↵ open · ? help", width: 80, styles: newStyles(true)}
	out := b.render()
	if strings.Contains(out, " / ") {
		t.Fatalf("directory bar should have no ' / ' separator: %q", out)
	}
	if !strings.Contains(out, "3 users") {
		t.Fatalf("bar %q missing meta", out)
	}
}

func TestStatusBarLandingShowsHint(t *testing.T) {
	out := landingBar(80, newStyles(true)).render()
	if !strings.Contains(out, "type a target") {
		t.Fatalf("landing bar %q missing hint", out)
	}
}

func TestStatusBarTruncatesBreadcrumbFirst(t *testing.T) {
	b := statusBar{host: "@an-extremely-long-hostname.example.org", user: "verylonguser",
		meta: "1.2 KB", hints: "esc back · ? help", width: 40, styles: newStyles(true)}
	out := b.render()
	if w := lipgloss.Width(out); w != 40 {
		t.Fatalf("bar width = %d, want 40 (must clamp)", w)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis when breadcrumb overflows: %q", out)
	}
	if !strings.Contains(out, "? help") {
		t.Fatalf("right-side hints must survive truncation: %q", out)
	}
}

func TestStatusBarShowsScrollAndPage(t *testing.T) {
	b := statusBar{host: "@tilde.team", user: "bob", scroll: "42%",
		hints: "? help", width: 80, styles: newStyles(true)}
	if !strings.Contains(b.render(), "42%") {
		t.Fatalf("bar missing scroll %%: %q", b.render())
	}
	b2 := statusBar{host: "@sdf.org", page: "page 2/4", meta: "42 users",
		hints: "? help", width: 80, styles: newStyles(true)}
	if !strings.Contains(b2.render(), "page 2/4") {
		t.Fatalf("bar missing page indicator: %q", b2.render())
	}
}

func TestStatusBarZeroWidthIsEmpty(t *testing.T) {
	if out := (statusBar{width: 0, styles: newStyles(true)}).render(); out != "" {
		t.Fatalf("zero-width bar = %q, want empty", out)
	}
}

func TestStatusBarWarnFlagRendered(t *testing.T) {
	b := statusBar{host: "@tilde.team", flags: []string{"partial (truncated)"},
		meta: "3 users", hints: "? help", width: 80, styles: newStyles(true)}
	if !strings.Contains(b.render(), "partial (truncated)") {
		t.Fatalf("bar missing warn flag")
	}
}

func TestStatusBarFlagNeverOverflowsNarrowWidth(t *testing.T) {
	for w := 1; w <= 30; w++ {
		b := statusBar{host: "@tilde.team", flags: []string{"partial (truncated)"},
			meta: "3 users", hints: "? help", width: w, styles: newStyles(true)}
		out := b.render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width %d: flagged bar wrapped to multiple lines:\n%q", w, out)
		}
		if lipgloss.Width(out) > w {
			t.Fatalf("width %d: flagged bar width = %d, exceeds limit", w, lipgloss.Width(out))
		}
	}
}

func TestStatusBarNeverWrapsAtNarrowWidth(t *testing.T) {
	// At a width too small for any gap, the bar must clip to one line, not wrap.
	for w := 1; w <= 30; w++ {
		b := statusBar{host: "@tilde.team", user: "jonathan", escTarget: "@tilde.team",
			meta: "1.2 KB", hints: "esc back · ? help", width: w, styles: newStyles(true)}
		out := b.render()
		if strings.Contains(out, "\n") {
			t.Fatalf("width %d: bar wrapped to multiple lines:\n%q", w, out)
		}
		if lipgloss.Width(out) > w {
			t.Fatalf("width %d: bar width = %d, exceeds limit", w, lipgloss.Width(out))
		}
	}
}

func TestStatusBarUsesBadgeStyle(t *testing.T) {
	st := newStyles(true)
	out := (statusBar{host: "@tilde.team", hints: "? help", width: 40, styles: st}).render()
	if strings.Contains(out, "STATUS") {
		t.Fatal("status bar should not invent a STATUS badge")
	}
	if !sameColor(st.barBadge.GetBackground(), st.palette.AccentViolet) {
		t.Fatal("bar badge should use accent violet")
	}
}
