# render/

Pure body rendering, no networking and no Bubble Tea. The repo-wide rules live
in the root `CLAUDE.md`; this file is the detail.

`Render(target, body, queryErr, profile) string` and `RenderWithBackground(target, body, queryErr, profile, darkBackground) string`. Metadata does not enter this layer: output starts on the first response line (or the empty-response/error treatment), with field highlighting but no synthetic receipt/header.

`RenderWithWidth(…, width)` is the same renderer with one addition: it wraps **the error line only** at `width` cells (0 = no wrap, which is what `RenderWithBackground` passes), so a long dial failure stays readable in a narrow terminal. Response bodies are never reflowed at any width — ASCII art and column layouts keep their exact bytes, and a clipped body line remains the deliberate trade (see issue #68).

This is the single rendering path for a finger response body, used by the TUI viewport (`render.RenderWithWidth` at the reader's viewport width, called from `tui/reader.go`, which re-renders on resize to re-wrap); `main.go` uses only `render.Usage`/`render.Version`/`render.ErrorLine` for `-h`/`-v`/startup-error text.

**Do not migrate this package to lipgloss v2.** It deliberately uses the **v1** `github.com/charmbracelet/lipgloss`; `.github/dependabot.yml` blocks the v1→v2 major bump so this can't drift. (The decision was evaluated and deferred on the cost of revalidating the golden corpus, not forgotten.)

Copy rendered here — usage text, error lines — is inventoried in
`docs/user-facing-messages.md`, a **living** doc: change the copy, change the
table in the same PR.
