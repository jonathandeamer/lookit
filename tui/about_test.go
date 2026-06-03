package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestAboutViewRendersIdentityAndActions(t *testing.T) {
	out := aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24)
	plain := stripANSIForLandingTest(out)
	for _, want := range []string{
		heroWordmark,
		aboutTagline,
		"lookit v0.0.1 · MIT license",
		"built 2026-06-03",
		aboutRepo,
		"Built with Charm · charm.sh",
		"young software — bug reports & ideas welcome",
		"finger jonathan@tilde.team",
		"↵ go",
		"report a bug or idea",
		"y copy",
		"thanks for supporting the small internet",
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
	plain := stripANSIForLandingTest(aboutView(newStyles(true), colorprofile.TrueColor, "dev", "unknown", 80, 24))
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
	wide := stripANSIForLandingTest(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 80, 24))
	narrow := stripANSIForLandingTest(aboutView(newStyles(true), colorprofile.TrueColor, "v0.0.1", "2026-06-03", 28, 24))
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
