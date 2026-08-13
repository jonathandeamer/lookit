# TUI visual review

Scripted stills of lookit's chrome, for a person or an agent to judge layout,
contrast, and copy. Not a demo, not a CI golden.

`demo/demo.tape` stays the README hero: window chrome, live hosts, Sleep
timings. These tapes are the opposite — no card, no network, `Wait` to prove
the state landed, a fixture bookmarks file, and one PNG per landed screen.

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

That builds `./lookit` and the loopback fingerd, starts the fingerd, and
records every `chrome-*.tape` and `responses-*.tape` into
`docs/tui-review/frames/<tape>/` — eight directories. Or record one tape:

```
make build review-fingerd
mkdir -p docs/tui-review/frames
vhs docs/tui-review/chrome-80-dark.tape
```

A `responses-*.tape` recorded by hand starts the fingerd itself; it needs
`make review-fingerd` first, and fails rather than record against a stranger
already holding 2479/2480 (`fingerd -ping` checks the fixture bodies, not just
that something answers).

Each recording starts from an empty `frames/` root and must produce exactly as
many stills as its tour has `Screenshot` lines, so a failed tape can't leave
frames behind for the next one to file under its own name. Every directory is
then checked by `verify-frames.sh` (blank stills, duplicate stills — see
"Why the tapes sleep before every Screenshot").

A full run takes about 3½ minutes. Frames are gitignored. Re-record when
chrome changes; do not commit PNGs.

## Why the tapes sleep before every Screenshot

Every still is `Wait` → `Sleep 1500ms` → `Screenshot`, and the order is load
bearing. VHS matches `Wait` against the live terminal but writes `Screenshot`
from a frame queue that lags behind it, so a `Screenshot` placed straight
after its `Wait` banks an *older* screen. It is not a race you notice by
looking at one frame: the first version of this kit shipped a blank `list.png`
and a `generic.png` showing the previous screen mid-keystroke, while the tape
exited 0.

Reproduce it in ten lines — `echo A; sleep 1; echo B`, `Wait+Screen /B/`,
`Screenshot`: the PNG shows `A`. A 500ms sleep is not always enough; 1s was
reliable in testing and 1500ms is the margin. Raising `Framerate` makes it
*worse* (more frames queued behind the encoder), so leave it at 12.

`verify-frames.sh` catches the two shapes this failure usually takes — a
blank still, and two stills identical to each other — but it cannot catch a
still that is a real screen one step behind. That one is on the sleeps, and on
you looking at the frames.

Tapes copy `fixtures/xdg/` to `frames/xdg/` and point `XDG_CONFIG_HOME` there,
so a developer's real bookmarks never appear in a frame *and* the stills that
write to the bookmarks file (`bookmark.png`, `catalog-off.png`) cannot dirty
the tracked fixture. The fixture ships one bookmark (`jonathan@tilde.team`) so
the BOOKMARKS section is on screen and the catalog stays visible; each
recording starts from that same state.

## Sizes and themes

Every size/theme is recorded twice — once for chrome (`chrome-*.tape`, offline)
and once for responses (`responses-*.tape`, loopback fingerd). Both families
share the same four geometries:

| Size | Cells | Why |
|---|---|---|
| `*-80-dark.tape` | 80×24 | classic terminal |
| `*-100-dark.tape` | 100×30 | startpage note column is designed at 100 columns |
| `*-60-dark.tape` | 60×20 | below `startWideMinWidth` (72); stacked layout |
| `*-80-light.tape` | 80×24 | AdaptiveColor on a light terminal background |

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
| `bookmark.png` | `b` on `@cosmic.voyage`: flash, BOOKMARKS 2, catalog count drops |
| `catalog-off.png` | `catalog off` in the file: BOOKMARKS is the whole page |

The last two mutate the throwaway bookmarks file, so they run last — every
earlier still shows the one-bookmark fixture state.

`responses-tour.tape` fingers the loopback fixture (`:2479` named, `:2480` generic), then a closed port.

| File | State |
|---|---|
| `list.png` | host listing at `@127.0.0.1:2479` (alice, bob) |
| `reader.png` | alice's .plan after Enter |
| `reader-link.png` | first link focused (`tab`) |
| `links.png` | `L` links panel |
| `raw.png` | `v` view source |
| `reader-input.png` | `i` on the landed reader (`esc cancel`) |
| `reader-help.png` | `?` panel over a landed reader — overlay vs the status bar |
| `reader-scroll.png` | `longplan` scrolled 12 lines; status carries a scroll percentage |
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

## Contact sheets

`make review-tui` finishes by writing one `frames/<tape>-sheet.png` per
directory: every still of that tape, half size, tiled four across. `make
review-sheet` alone rebuilds them from frames already on disk. Cells are in
filename order and the target prints the manifest for each sheet as it goes.

The sheet is an index, not the review — read it to find the frames worth
opening at full size, then open those. Eight sheets is a practical first pass
where 76 individual stills is not.

## Agent review

1. `make review-tui` (or record the tape that matches the change).
2. Read the eight contact sheets first to triage, then open the full-size
   PNGs that look wrong. They are images; there is no GIF to review.
3. Work the rubric against the scene list above.
4. Visit every size/theme that shares the chrome you touched.
5. Leave `View()` tests and contrast tests as the regression net.

Do not drive `./lookit` from this harness expecting a TTY — `tea.NewProgram`
needs a real terminal, which VHS provides and the agent sandbox does not.
