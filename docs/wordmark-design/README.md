# lookit wordmark

An ASCII/Unicode wordmark for splash use. The two `o`s in **l‑oo‑kit** sit exactly
where a pair of eyes belongs, so they're drawn as eyeballs glancing up‑left — the
👀 emoji, rendered in block art.

## Hero

`wordmark.txt` (38 cols × 6 rows):

```
██    ▄████▄   ▄████▄  ██     ██  ██
██   ████████ ████████ ██        █████
██   ██▓▓████ ██▓▓████ ██ ██  ██  ██
██   ██▓▓████ ██▓▓████ ████   ██  ██
██   ████████ ████████ ██ ██  ██  ██
████  ▀████▀   ▀████▀  ██  ██ ██  ███
```

The `▓▓` block in the upper‑left of each eyeball is the pupil — offset so both
eyes glance the same way, which is what makes a flat disc read as a *looking* eye
rather than a dot. `wordmark.ans` is the same art pre‑coloured with the palette
below; `cat docs/wordmark-design/wordmark.ans` to preview it in a truecolor
terminal.

## Files

| File | What | Size |
|------|------|------|
| `wordmark.txt` | Hero, plain (embed this) | 38×6 |
| `wordmark.ans` | Hero, pre‑coloured ANSI (truecolor, dark bg) | 38×6 |
| `wordmark-mini.txt` | Narrow‑terminal fallbacks (3‑row + one‑liners) | ≤13 wide |
| `alternates.txt` | Two other weights (outline, thin) if the hero is too heavy | — |

## Colour treatment

Uses lookit's own palette (see `tui/styles.go` / `render/theme.go`) so the splash
matches the rest of the app. Every region is **adaptive** — paired light/dark
values — per the project's colour rule, so light terminals (Terminal.app default)
stay legible:

| Region | Palette field | Dark hex | Light hex |
|--------|---------------|----------|-----------|
| Letters `l k i t` | `AccentViolet` | `#9878ff` | `#6d43d6` |
| Eyeball (sclera) | `Text` | `#f0edf5` | `#25222a` |
| Pupil `▓▓` | `AccentPink` | `#ff5fa2` | `#c92870` |

On a dark terminal that's white eyeballs with pink pupils; on a light terminal the
eyeballs invert to near‑black (still ringed and reading as eyes), pupils stay pink,
letters stay violet. Want the eyes to pop more? Recolour one pupil cell to
`AccentGold` (`#eed76d`) as a glint.

## Notes for splash use

- **All glyphs are width‑1.** Built from block elements (`█▀▄▓`) and box‑drawing
  only — no geometric/emoji codepoints (`●◕👀`), which are East‑Asian *ambiguous*
  or double width and would shear the columns in some terminals.
- **Degrades to mono.** With no colour the `▓` pupils still shade differently from
  the `█` sclera, so the eyes survive a `NO_COLOR` / ASCII profile. Pair this with
  the existing `colorprofile` gate (plain output when not a TTY).
- **Centre, don't left‑pad.** Measure with `lipgloss.Width` and place; the trailing
  spaces in the art are already trimmed to a clean 38‑col box.
- Below ~40 cols, drop to `wordmark-mini.txt`.

## Using it on a splash screen

Drop something like this into `tui/` (Bubble Tea v2 / `charm.land/lipgloss/v2`).
It colours by column region — the eyeball columns are fixed by the layout — and
returns plain art when colour is off. Call it from your splash model's `View`.

```go
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// wordmark is docs/wordmark-design/wordmark.txt, verbatim.
const wordmark = "" +
	"██    ▄████▄   ▄████▄  ██     ██  ██  \n" +
	"██   ████████ ████████ ██        █████\n" +
	"██   ██▓▓████ ██▓▓████ ██ ██  ██  ██  \n" +
	"██   ██▓▓████ ██▓▓████ ████   ██  ██  \n" +
	"██   ████████ ████████ ██ ██  ██  ██  \n" +
	"████  ▀████▀   ▀████▀  ██  ██ ██  ███ "

// eyeball columns in the art (half-open ranges); everything else is a letter.
var eyeCols = [][2]int{{5, 13}, {14, 22}}

func inEye(c int) bool {
	for _, r := range eyeCols {
		if c >= r[0] && c < r[1] {
			return true
		}
	}
	return false
}

// renderWordmark colours the hero with the palette. When noColor is true it
// returns the raw block art (still readable — the ▓ pupils self-shade).
func renderWordmark(p palette, noColor bool) string {
	if noColor {
		return wordmark
	}
	letter := lipgloss.NewStyle().Foreground(p.AccentViolet)
	sclera := lipgloss.NewStyle().Foreground(p.Text)
	pupil := lipgloss.NewStyle().Foreground(p.AccentPink)

	var out strings.Builder
	for _, line := range strings.Split(wordmark, "\n") {
		for c, r := range []rune(line) {
			s := string(r)
			switch {
			case r == ' ':
				out.WriteString(s)
			case r == '▓':
				out.WriteString(pupil.Render(s))
			case inEye(c):
				out.WriteString(sclera.Render(s))
			default:
				out.WriteString(letter.Render(s))
			}
		}
		out.WriteByte('\n')
	}
	return out.String()
}

// Splash centres the wordmark (and an optional tagline) in the viewport.
func Splash(width, height int, p palette, noColor bool) string {
	art := renderWordmark(p, noColor)
	if width < 40 { // too narrow for the hero — see wordmark-mini.txt
		art = "l(o)(o)kit"
	}
	tagline := lipgloss.NewStyle().Foreground(p.Dim).Render("finger, for the modern terminal")
	block := lipgloss.JoinVertical(lipgloss.Center, art, "", tagline)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}
```

`palette` already carries `AccentViolet`, `AccentPink`, `Text`, and `Dim` (see
`paletteFor` in `tui/styles.go`), and both are built from `lipgloss.HasDarkBackground()`,
so the adaptive light/dark behaviour comes for free. This file only holds the
design assets — wiring an actual splash state into `appModel` is a separate change.

## Alternates

If the block hero is too heavy for a given context, `alternates.txt` has:

- **Outline** — lighter rounded box‑drawing, ~26×5.
- **Thin** — classic figlet `doom` pipes with `(o_)` / `(___)` eyes, pure ASCII, ~27×6.
