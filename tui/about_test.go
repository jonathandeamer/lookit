package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestAboutViewRendersIdentityAndActions(t *testing.T) {
	out := aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24)
	plain := ansi.Strip(out)
	for _, want := range []string{
		heroWordmark,
		aboutTagline,
		"lookit v0.0.1 · MIT license",
		"built 2026-06-03",
		aboutRepo,
		"Built with Charm https://charm.sh",
		"Young software; bug reports & ideas welcome",
		"finger jonathan@tilde.team",
		"↵ go",
		"Report a bug or idea",
		"y copy issues URL",
		"Thanks for supporting the small internet",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("about view missing %q:\n%s", want, plain)
		}
	}
	for _, bad := range []string{"RFC 1288", "telemetry", "read-only"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("about view should not contain %q:\n%s", bad, plain)
		}
	}
}

func TestAboutViewHidesBuildRowWhenUnknown(t *testing.T) {
	plain := ansi.Strip(aboutView(newStyles(true), colorprofile.TrueColor, "dev", "unknown", 80, 24))
	if strings.Contains(plain, "built ") {
		t.Fatalf("about view should hide the build row when builtAt is unknown:\n%s", plain)
	}
	if !strings.Contains(plain, "lookit dev · MIT license") {
		t.Fatalf("about view should still show the dev version line:\n%s", plain)
	}
}

func TestAboutViewHeroGradientByProfile(t *testing.T) {
	// aboutView delegates the hero's gradient/solid choice to headerMark via the
	// profile; the non-hero palette lines are identical across profiles. So the
	// truecolor view (per-rune gradient) must show strictly more distinct colours
	// than the ANSI view (solid wordmark fallback).
	tc := len(foregroundSequences(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24)))
	an := len(foregroundSequences(aboutView(newStyles(true), colorprofile.ANSI, "v0.0.1", "2026-06-03", 80, 24)))
	if tc <= an {
		t.Fatalf("truecolor about view should use more distinct colours (gradient) than ANSI (solid): tc=%d an=%d", tc, an)
	}
}

func TestAboutViewNarrowTruncatesLongLines(t *testing.T) {
	wide := ansi.Strip(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24))
	narrow := ansi.Strip(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 28, 24))
	if !strings.Contains(narrow, heroWordmark) {
		t.Fatalf("narrow about view should still show the wordmark:\n%s", narrow)
	}
	if strings.Contains(narrow, aboutTagline) {
		t.Fatalf("narrow about view should truncate the long tagline:\n%s", narrow)
	}
	if !strings.Contains(wide, aboutTagline) {
		t.Fatalf("wide about view should show the full tagline:\n%s", wide)
	}
}

// Fitting after centering used to truncate JoinVertical's right-hand padding,
// marking every line as truncated and pushing the block off-centre.
func TestAboutViewNarrowOnlyEllipsizesOverlongLines(t *testing.T) {
	const width = 60
	plain := ansi.Strip(aboutView(newStyles(true), colorprofile.TrueColor, "v0.2.0", "2026-08-14", width, 24))
	for _, ln := range strings.Split(plain, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if ansi.StringWidth(ln) > width {
			t.Fatalf("line exceeds width %d: %q", width, ln)
		}
		// A genuine overflow is width cells of content ending in "…". Interior
		// pad before the ellipsis is the old truncate-after-center bug.
		trimmed := strings.TrimRight(ln, " ")
		if !strings.HasSuffix(trimmed, "…") {
			continue
		}
		if ansi.StringWidth(trimmed) != width {
			t.Fatalf("ellipsized line is not %d cells of content: %q", width, ln)
		}
		content := strings.TrimSuffix(trimmed, "…")
		if content != strings.TrimRight(content, " ") {
			t.Fatalf("ellipsis after interior pad at width %d: %q", width, ln)
		}
	}
	if !strings.Contains(plain, aboutTagline) {
		t.Fatalf("tagline fits in %d columns and should not be truncated:\n%s", width, plain)
	}
	// The wordmark is far shorter than the width, so it must sit centred rather
	// than be dragged left by an overlong sibling line.
	for _, ln := range strings.Split(plain, "\n") {
		if !strings.Contains(ln, heroWordmark) {
			continue
		}
		lead := ansi.StringWidth(ln) - ansi.StringWidth(strings.TrimLeft(ln, " "))
		trail := ansi.StringWidth(ln) - ansi.StringWidth(strings.TrimRight(ln, " "))
		if lead-trail > 1 || trail-lead > 1 {
			t.Fatalf("wordmark off-centre at width %d (lead=%d trail=%d): %q", width, lead, trail, ln)
		}
		return
	}
	t.Fatalf("wordmark line not found:\n%s", plain)
}

// Both credits render as bullets carrying an OSC 8 hyperlink, and neither is
// left in the dim identity block.
func TestAboutViewRendersCreditBulletsWithHyperlinks(t *testing.T) {
	out := aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24)
	plain := ansi.Strip(out)
	for _, tc := range []struct{ label, url string }{
		{"Built with Charm", aboutCharmURL},
		{"Catalog inspired by", aboutCatalogCreditURL},
	} {
		wantLine := "✦ " + tc.label + " " + tc.url
		if !strings.Contains(plain, wantLine) {
			t.Fatalf("about view missing credit bullet %q:\n%s", wantLine, plain)
		}
		if !strings.Contains(out, "\x1b]8;;"+tc.url) {
			t.Fatalf("about view missing OSC 8 hyperlink for %s:\n%q", tc.url, out)
		}
	}
}

// The catalog credit sits below the personality line, so the longest URL lands
// at the bottom edge of the bullet group.
func TestAboutViewCatalogCreditIsTheLastBullet(t *testing.T) {
	plain := ansi.Strip(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24))
	charm := strings.Index(plain, "Built with Charm")
	young := strings.Index(plain, "Young software")
	catalog := strings.Index(plain, "Catalog inspired by")
	if charm >= young || young >= catalog {
		t.Fatalf("expected bullet order Charm < Young < Catalog, got %d/%d/%d:\n%s", charm, young, catalog, plain)
	}
	repo := strings.Index(plain, aboutRepo)
	if repo > charm {
		t.Fatalf("repo line should stay above the credit bullets, got repo=%d charm=%d:\n%s", repo, charm, plain)
	}
}
