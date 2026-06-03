# About screen design

## Goal

Give lookit a dedicated **about screen** — a full-screen view, opened with `a`,
that is the home for the app's identity and personality. It consolidates branding
and metadata that currently live in two thinner places (the `☞ lookit` wordmark
above the landing's `target:` row, and the version line tucked into the `?` help
panel) and adds the warmth appropriate to an early release: a Charm credit, an
honest "young software" note, and — most on-brand for a finger client — a way to
**finger the author from inside the app**.

This is deliberate: an about screen is where a finger client gets to have a soul,
and it is the natural home for the protocol-education and warmth we kept out of
the slim help panel and the bare landing.

## Non-goals

- **No theme, config, or palette changes.** Colour comes entirely from the
  existing fixed palette and adaptive light/dark theming.
- **No new network behaviour** beyond the user-initiated author finger, which is
  an ordinary lookup through the existing fetch path. The screen renders offline;
  nothing is dispatched until the user presses `↵`.
- **No persistent state.** The about screen is transient: it is not pushed onto
  the history stack, and `esc` returns to exactly the screen it was opened from.
- **No figlet / ASCII banner.** The hero is the existing `☞ lookit` manicule +
  per-rune gradient wordmark (`headerMark`), not a wide block-letter banner.
- **No commit hash or Go-version line.** The version block shows version, build
  date, and licence only.
- **No mouse capture** (consistent with the rest of the TUI).

## Behaviour

### Landing (the wordmark moves out)

The `☞ lookit` mark is removed from the normal app chrome. `inputChromeView`
renders only `m.input.View()`, so the focused landing is just the `target:`
prompt, and `topChromeHeight()` is always `1`. The mark is removed everywhere the
input is focused (the launch landing *and* re-focusing the input on a result
screen), which is consistent: the wordmark now lives only on the about screen.
`headerMark`/`gradientColors` are retained — the about hero reuses them. This is
the splash-removal endpoint the landing-hero spec anticipated ("the durable asset
is the finger mark plus per-rune `lookit` gradient").

### Opening and closing About (honest keybinding)

The target input is focused on the landing, and the key router only intercepts
`?`, `↵`, and `esc` while the input is focused — **every other printable key,
including `a`, types into the field** (so a user can type `alice@host`). Therefore
`a` cannot be a global "open about" key: advertising it on the focused landing
would be dishonest (it would type `a`, exactly as `q` does), which the project's
honesty convention forbids.

So About is reached two honest ways, matching the "option `a` in the help panel"
framing:

- **Blurred result screens** (reader / list, not filtering): `a` opens About
  directly, joining the `i`/`v`/`y`/`q` content-key family.
- **Landing / focused input:** `?` opens help (it works while focused), the help
  panel lists **`a · about lookit`**, and pressing `a` *while the help panel is
  open* opens About. The discovery path is `?` → `a`.

The landing status-bar hint is unchanged and stays honest:
`type a target and press ↵ · ? help`. It never advertises `a about`.

Closing: on the about screen, `esc` (or `a`) returns to the screen About was
opened from, with no re-fetch; `q` quits; `ctrl+c` always quits.

### Actions on the about screen

- **`↵` fingers the author.** Synthesizes the target `jonathan@tilde.team`
  (already a blessed sample target) and dispatches the ordinary fetch command, so
  the result flows through the standard `routeFetch` → loading → reader path and
  is pushed onto history like any lookup. About is left as the fetch starts;
  `esc` from the author's profile then follows normal history back-navigation.
  The author finger is treated as a user-initiated lookup (not a server-supplied
  target), so the user-typed-port preservation rules are irrelevant here — it is
  a fixed `:79` finger like any sample.
- **`y` copies the issues URL.** Copies `https://github.com/jonathandeamer/lookit/issues`
  to the clipboard via the existing OSC-52 `setClipboard`, with a `copied …`
  flash, mirroring `copyAddress`.

## Content and exact copy

The body is a single block, centered vertically and horizontally in the region
between the top of the screen and the status bar. Context and key hints live in
the status bar, consistent with lookit's "status bar owns the breadcrumb + hints"
model (so the body carries no top chrome row of its own).

```
                      ☞ lookit                         gradient hero (headerMark)
        A modern TUI browser for the Finger protocol   tagline · Dim

              lookit v0.0.1 · MIT license             version + licence · Dim
                    built 2026-06-03                   build date · Dim
              github.com/jonathandeamer/lookit         repo

            ✦ Built with Charm · charm.sh              credit (charm.sh names the toolkit)
            ✦ young software — bug reports & ideas welcome   honest early-release

      ➜ finger jonathan@tilde.team        ↵ go         centerpiece (actionable)
      ➜ report a bug or idea              y copy       copies the issues URL

        thanks for supporting the small internet       warmth
```

Status bar while About is open:

- **Left:** `◂ esc: <previous target>` when opened from a result screen (the
  honest "esc goes back, and where to" signal), or `about lookit` when opened
  from the bare landing (no previous target).
- **Right (hints):** `↵ go · y copy · esc back · q quit`.

Copy notes: the `v0.0.1` and `built 2026-06-03` shown above are illustrative —
version and build date are the injected build values (ldflags, with the
`ReadBuildInfo` fallback below). "Built with Charm · charm.sh" credits the Charm
TUI toolkit (Bubble Tea / Bubbles / Lip Gloss) and points at `charm.sh` so the
name is self-explanatory. "young software" labels the early-release stage plainly.
The licence reads `MIT license` (repo `LICENSE`, © 2026 Jonathan Deamer). The
protocol pointer (`finger · RFC 1288`) and the "read-only · no telemetry" trust
line are deliberately dropped: the tagline already names the protocol, and for a
tool this small and obviously transparent the trust line states the obvious and
reads faintly defensive, against the smallnet/anti-corporate register.

## Architecture

A new `stateAbout` joins the `appState` enum beside `stateReader` and
`stateList` — the soft-serve "page" pattern and the charm-native full-screen
route (glow's full help and soft-serve's pages take over the screen; neither uses
a floating modal, so we don't either, per lookit's "what does glow do?" rule).
The lipgloss v2 `Layer`/`Compositor` modal primitive is deliberately **not**
adopted.

- **`tui/about.go` (new):** an `aboutModel` sub-model mirroring `readerModel`. It
  holds `width`/`height`/`profile`/`darkBackground`/`styles`/`version` and the
  static content, owns no lifecycle/quit (appModel drives it via `setSize`/
  `setProfile`/`setBackground`), and exposes a **pure `View()`** built from a pure
  `aboutView(...) string` helper centered with `lipgloss.Place` /
  `PlaceVertical`. String in, string out — golden-testable, like `heroView`/
  `render.Render`.
- **`tui/app.go`:**
  - `stateAbout` enum value; a `View()` branch rendering `m.about.View()`.
  - `openAbout`/`closeAbout` helpers. `openAbout` stashes the origin appState (and
    whether the input was focused / `pos < 0`) so `closeAbout` restores it without
    a re-fetch; About is not pushed onto `history`.
  - `handleKey`: an About `case` is added in **two** places only — the help-open
    block (so `a` opens About instead of closing the panel; all other keys still
    close it) and the content/blurred block. It is **never** added to the
    focused-input block, which is what keeps `a` typeable on the landing. A
    `stateAbout` key block handles `↵` (finger author), `y` (copy issues),
    `esc`/`a` (return), `q` (quit).
  - `inputChromeView` drops the `headerMark` row; `topChromeHeight()` returns `1`.
  - The status bar gains an About variant (left context + the hints above).
- **`tui/keys.go`:** an `About` binding (`key.WithKeys("a")`,
  `key.WithHelp("a", "about lookit")`); `updateKeymap` sets it
  `SetEnabled(true)` unconditionally (like `Help`) so it always appears in the
  panel and is matchable in the help-open and content branches; `FullHelp` lists
  it.
- **`main.go`:** adopt the crush/soft-serve/wishlist `debug.ReadBuildInfo()`
  idiom as a fallback to the `-ldflags` `version`/`builtAt` vars, so
  `go install …@latest` (no ldflags) shows the module version (`info.Main.Version`)
  and, when `builtAt` is unset, the VCS commit time (`vcs.time`) for the build
  date — without rendering a commit hash. Releases keep the ldflags values.

No change to `routeFetch`, `ParseUsers`, `finger/`, or `render/`. The reader/list
sub-models are untouched.

## Degradation and edge cases

- **Gradient by profile.** The hero reuses `headerMark`, so TrueColor/ANSI256 get
  the per-rune pink→violet→mint gradient and ANSI (16-colour) and below fall back
  to a solid bold `AccentViolet` wordmark. The gradient is decorative; the
  wordmark and all text are always legible.
- **Glyph widths.** `☞`, `✦`, and `➜` widths are measured with `lipgloss.Width`
  so centering stays correct regardless of how a terminal widths them.
- **Narrow terminals.** Lines truncate with `…` (longest first — tagline, repo
  URL); the block stays centered within the available width.
- **Very short terminals.** When the body height is too small to center
  comfortably, the block top-aligns within the body rather than clipping.
- **Background change.** A `tea.BackgroundColorMsg` while About is open restyles
  it through the shared `styles`, like the rest of the chrome.

## Testing

Offline, no real TTY, injected fetch — following the existing patterns.

- **Pure `aboutView` goldens** across profiles (TrueColor / ANSI256 / ANSI) and
  both background modes, plus a narrow (`< 40` col) case asserting truncation and
  that the wordmark + action lines survive.
- **Transitions:** About opens via `?`→`a` from the landing and via `a` from a
  blurred result; `↵` dispatches a fetch for `jonathan@tilde.team`; `y` issues a
  clipboard command and sets the flash; `esc`/`a` restore the origin state with
  no re-fetch; About never enters `history`.
- **Honesty:** pressing `a` while the input is focused on the landing types `a`
  into the field (does **not** open About); the landing hint never contains
  `a about`.
- **Version fallback:** `ReadBuildInfo` fills the module version when ldflags is
  unset; no commit hash appears in the rendered version.
- `make check` is the final gate.

## Accepted residual risk

- Perceived gradient colours after ANSI256/ANSI downsampling vary by terminal
  theme — controlled by testing truecolour source stops and the legible solid
  fallback, consistent with the existing theming pass.
- `☞`/`✦`/`➜` rendering width varies by font — mitigated by measuring with
  `lipgloss.Width` rather than assuming fixed columns.
- The author finger (`jonathan@tilde.team`) depends on tilde.team being
  reachable; if it is down the lookup errors like any finger target (honest, not
  special-cased).
