package render

import (
	"image/color"
	"math"
	"reflect"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestRenderPaletteContrast(t *testing.T) {
	tests := []struct {
		name  string
		fg    color.Color
		bg    color.Color
		ratio float64
	}{
		{"dark text", renderPaletteFor(true).Text, renderPaletteFor(true).BaseBg, 4.5},
		{"dark dim", renderPaletteFor(true).Dim, renderPaletteFor(true).BaseBg, 4.5},
		{"dark field", renderPaletteFor(true).AccentPink, renderPaletteFor(true).BaseBg, 4.5},
		{"dark target", renderPaletteFor(true).AccentPink, renderPaletteFor(true).BaseBg, 4.5},
		{"dark warning", renderPaletteFor(true).AccentGold, renderPaletteFor(true).BaseBg, 4.5},
		{"dark error", renderPaletteFor(true).AccentRed, renderPaletteFor(true).BaseBg, 4.5},
		{"light text", renderPaletteFor(false).Text, renderPaletteFor(false).BaseBg, 4.5},
		{"light dim", renderPaletteFor(false).Dim, renderPaletteFor(false).BaseBg, 4.5},
		{"light field", renderPaletteFor(false).AccentPink, renderPaletteFor(false).BaseBg, 4.5},
		{"light target", renderPaletteFor(false).AccentPink, renderPaletteFor(false).BaseBg, 4.5},
		{"light warning", renderPaletteFor(false).AccentGold, renderPaletteFor(false).BaseBg, 4.5},
		{"light error", renderPaletteFor(false).AccentRed, renderPaletteFor(false).BaseBg, 4.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contrastRatio(tc.fg, tc.bg); got < tc.ratio {
				t.Fatalf("contrast %.2f below %.2f", got, tc.ratio)
			}
		})
	}
}

func TestRenderThemeLightDarkColoursDiffer(t *testing.T) {
	dark := NewTheme(colorprofile.TrueColor, true)
	light := NewTheme(colorprofile.TrueColor, false)
	if sameColor(dark.Field.GetForeground(), light.Field.GetForeground()) {
		t.Fatal("field foreground should differ between dark and light backgrounds")
	}
	if sameColor(dark.Warning.GetForeground(), light.Warning.GetForeground()) {
		t.Fatal("warning foreground should differ between dark and light backgrounds")
	}
}

func contrastRatio(a, b color.Color) float64 {
	l1, l2 := relativeLuminance(a), relativeLuminance(b)
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.2126*linear(float64(r)/65535) +
		0.7152*linear(float64(g)/65535) +
		0.0722*linear(float64(b)/65535)
}

func linear(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func sameColor(a, b color.Color) bool {
	return reflect.DeepEqual(a, b)
}
