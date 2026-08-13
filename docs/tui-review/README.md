# TUI visual review

Scripted stills of lookit's chrome, for a person or an agent to judge layout,
contrast, and copy. Not a demo, not a CI golden.

`demo/demo.tape` stays the README hero: window chrome, live hosts, Sleep
timings. These tapes are the opposite — no card, no network, `Wait` not
`Sleep`, a fixture bookmarks file, and one PNG per landed screen.

`make check` does not run this. Contrast and `View()` assertions stay in the
Go tests; this pass is "does this look wrong." List/reader stills come from a
loopback fingerd (`127.0.0.1:2479` named listing, `:2480` generic);
error stills dial a closed port. Nothing here leaves the machine.

## Record

Needs `vhs`, `ttyd`, and `ffmpeg` (`brew install vhs ffmpeg ttyd`) and the
JetBrainsMono Nerd Font (`brew install --cask font-jetbrains-mono-nerd-font`)
so the wordmark renders. From the repo root:

```
make review-tui
```

That builds `./lookit` and records every `chrome-*.tape` into
`docs/tui-review/frames/<tape>/`. Or record one tape:

```
make build
mkdir -p docs/tui-review/frames
vhs docs/tui-review/chrome-80-dark.tape
```

Frames are gitignored. Re-record when chrome changes; do not commit PNGs.

Tapes pin `XDG_CONFIG_HOME` to `fixtures/xdg/` so a developer's real bookmarks
never appear in a frame. The fixture ships one bookmark (`jonathan@tilde.team`)
so the BOOKMARKS section is on screen and the catalog stays visible.

## Sizes and themes

| Tape | Cells | Why |
|---|---|---|
| `chrome-80-dark.tape` | 80×24 | classic terminal |
| `chrome-100-dark.tape` | 100×30 | startpage note column is designed at 100 columns |
| `chrome-60-dark.tape` | 60×20 | below `startWideMinWidth` (72); stacked layout |
| `chrome-80-light.tape` | 80×24 | AdaptiveColor on a light terminal background |

Pixel sizes are calibrated at `FontSize 18` and `Padding 0` (VHS `Set Width`
is pixels, not columns). Re-calibrate with `tput cols; tput lines` if the
font size changes.

## Scenes

`tour.tape` is the chrome walkthrough. It never fetches.

| File | State |
|---|---|
| `start-input.png` | startpage, target input focused |
| `start-list.png` | startpage, first row selected (bookmark) |
| `start-child.png` | SERVICES child `dict` selected; note and `├` visible |
| `start-filter.png` | startpage flattened by `/plan` |
| `help.png` | `?` panel on the startpage |
| `about.png` | About (`a` from the open help panel) |

`responses-tour.tape` fingers the loopback fixture (`:2479` named, `:2480` generic), then a closed port.

| File | State |
|---|---|
| `list.png` | host listing at `@127.0.0.1:2479` (alice, bob) |
| `reader.png` | alice's .plan after Enter |
| `reader-link.png` | first link focused (`tab`) |
| `links.png` | `L` links panel |
| `raw.png` | `v` view source |
| `reader-input.png` | `i` on the landed reader (`esc cancel`) |
| `generic.png` | `@127.0.0.1:2480` — `auto-detected` |
| `truncated.png` | `trunc@127.0.0.1:2479` — `partial (truncated)` |
| `error.png` | `nobody@127.0.0.1:1` — dial refused, `r retry` (Wait matches `connect:` so 60-col wrap still lands) |

## Rubric

Review the PNGs as images. One happy-path frame is not a review. Walk every
tape and hunt collisions, not just whether the screen rendered.

1. Charm / bubbles defaults, then vim flavour, then smallnet convention.
2. Copy is honest — no invented protocol structure; derived facts carry
   meaning.
3. Light and dark both readable. Pair every colour; `TestTUIPaletteContrast`
   already checks ratios, this pass checks composition.
4. macOS-first: no Option chords implied on screen, no mouse capture, native
   select/copy still possible.
5. Specific hunts: status hints vs available width, help overlay vs the bar,
   note column at 100 columns and crush at 60, BOOKMARKS vs catalog hierarchy,
   `├`/`└` children, selection shelf, gradient wordmark, input placeholder
   (`user@host or @host`) vs the list below it.

## Agent review

1. `make review-tui` (or record the tape that matches the change).
2. Open each PNG (they are images; do not review the GIF).
3. Work the rubric against the scene list above.
4. Visit every size/theme that shares the chrome you touched.
5. Leave `View()` tests and contrast tests as the regression net.

Do not drive `./lookit` from this harness expecting a TTY — `tea.NewProgram`
needs a real terminal, which VHS provides and the agent sandbox does not.
