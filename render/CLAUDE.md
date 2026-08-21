# render/

Pure body rendering, no networking and no Bubble Tea. The repo-wide rules live
in the root `CLAUDE.md`; this file is the detail.

`Render(target, body, queryErr, profile) string` and `RenderWithBackground(target, body, queryErr, profile, darkBackground) string`. Metadata does not enter this layer: output starts on the first response line (or the empty-response/error treatment), with field highlighting but no synthetic receipt/header.

`RenderWithWidth(…, width)` remains the compatibility renderer: it wraps the error line only at `width` cells and never reflows the response body. `RenderLayout` is the explicit TUI layout path; when given a positive body width it wraps each existing physical body line independently at whitespace, never splits an overlong token, and returns a display-row-to-logical-body-line map. A tab-bearing or whitespace-only physical line passes through intact, and non-breaking space is not a break opportunity. Error text still wraps independently at the full viewport width.

This is the single rendering path for a finger response body, used by the TUI viewport (`render.RenderLayout`, called from `tui/reader.go`); `main.go` uses only `render.Usage`/`render.Version`/`render.ErrorLine` for `-h`/`-v`/startup-error text.

**Do not migrate this package to lipgloss v2.** It deliberately uses the **v1** `github.com/charmbracelet/lipgloss`; `.github/dependabot.yml` blocks the v1→v2 major bump so this can't drift. (The decision was evaluated and deferred on the cost of revalidating the golden corpus, not forgotten.)

Copy rendered here — usage text, error lines — is inventoried in
`docs/user-facing-messages.md`, a **living** doc: change the copy, change the
table in the same PR.
