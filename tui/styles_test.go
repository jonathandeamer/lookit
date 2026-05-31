package tui

import (
	"image/color"
	"math"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTUIPaletteContrast(t *testing.T) {
	tests := []struct {
		name string
		p    palette
	}{
		{name: "dark", p: paletteFor(true)},
		{name: "light", p: paletteFor(false)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContrast(t, "text on base", tt.p.Text, tt.p.BaseBg, 4.5)
			assertContrast(t, "dim on base", tt.p.Dim, tt.p.BaseBg, 4.5)
			assertContrast(t, "pink on base", tt.p.AccentPink, tt.p.BaseBg, 4.5)
			assertContrast(t, "violet on base", tt.p.AccentViolet, tt.p.BaseBg, 4.5)
			assertContrast(t, "mint on base", tt.p.AccentMint, tt.p.BaseBg, 4.5)
			assertContrast(t, "gold on base", tt.p.AccentGold, tt.p.BaseBg, 4.5)
			assertContrast(t, "red on base", tt.p.AccentRed, tt.p.BaseBg, 4.5)
			assertContrast(t, "status text on subtle bg", tt.p.BarText, tt.p.SubtleBg, 4.5)
			assertContrast(t, "help key on subtle bg", tt.p.Text, tt.p.SubtleBg, 4.5)
			assertContrast(t, "help desc on subtle bg", tt.p.BarText, tt.p.SubtleBg, 4.5)
			assertContrast(t, "selected login on selected bg", tt.p.SelectionLogin, tt.p.SelectionBg, 4.5)
			assertContrast(t, "selected desc on selected bg", tt.p.SelectionDesc, tt.p.SelectionBg, 4.5)
			assertContrast(t, "selection rail on selected bg", tt.p.AccentViolet, tt.p.SelectionBg, 3.0)
		})
	}
}

func TestNewStylesUsesFunctionalBrightRoles(t *testing.T) {
	st := newStyles(true)
	p := paletteFor(true)

	assertSameColor(t, "barBadge background", st.barBadge.GetBackground(), p.AccentViolet)
	assertSameColor(t, "spinner foreground", st.spinner.GetForeground(), p.AccentMint)
	assertSameColor(t, "selected title border", st.listItem.SelectedTitle.GetBorderLeftForeground(), p.AccentViolet)
	assertSameColor(t, "selected title background", st.listItem.SelectedTitle.GetBackground(), p.SelectionBg)
	assertSameColor(t, "selected title foreground", st.listItem.SelectedTitle.GetForeground(), p.SelectionLogin)
}

func TestHexColorAcceptsFunctionalBrightValues(t *testing.T) {
	assertSameColor(t, "functional bright violet", hexColor("#8a63ff"), lipgloss.Color("#8a63ff"))
}

func assertContrast(t *testing.T, name string, fg, bg color.Color, minimum float64) {
	t.Helper()
	if got := contrastRatio(fg, bg); got < minimum {
		t.Fatalf("%s contrast = %.2f, want >= %.2f", name, got, minimum)
	}
}

func contrastRatio(fg, bg color.Color) float64 {
	l1 := relativeLuminance(fg)
	l2 := relativeLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.2126*linearizedChannel(r) + 0.7152*linearizedChannel(g) + 0.0722*linearizedChannel(b)
}

func linearizedChannel(v uint32) float64 {
	c := float64(v) / math.MaxUint16
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func assertSameColor(t *testing.T, name string, got, want color.Color) {
	t.Helper()
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()
	if gr != wr || gg != wg || gb != wb || ga != wa {
		t.Fatalf("%s = rgba(%d,%d,%d,%d), want rgba(%d,%d,%d,%d)", name, gr, gg, gb, ga, wr, wg, wb, wa)
	}
}
