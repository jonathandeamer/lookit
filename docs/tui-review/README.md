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
`out/tui-review/<tape>/` — twelve directories. Or record one tape:

```
make build review-fingerd
mkdir -p out/tui-review
vhs docs/tui-review/chrome-80-dark.tape
```

A `responses-*.tape` recorded by hand starts the fingerd itself; it needs
`make review-fingerd` first, and fails rather than record against a stranger
already holding 2479/2480 (`fingerd -ping` checks the fixture bodies, not just
that something answers).

Each recording starts from an empty `out/tui-review/` root and must produce
exactly as many stills as its tour has `Screenshot` lines, so a failed tape
can't leave frames behind for the next one to file under its own name. Every
directory is then checked by `verify-frames.sh` (blank stills, duplicate
stills — see "Why the tapes sleep before every Screenshot").

A full run takes about seven minutes. Everything it writes lands in `out/`,
which is gitignored wholesale — nothing generated is ever interleaved with the
tapes and fixtures here, and `make clean` removes all of it. Re-record when
chrome changes; do not commit PNGs.

### One tape failing does not end the run

`record-tape.sh` records a single tape and `make review-tui` calls it once per
tape, **carrying on when one fails**. The run still builds the contact sheets,
then lists the tapes that did not record and exits non-zero — as it also does
when the sheets themselves fail, even with every tape recorded — so a tape
that drops out at slot 10 of 12 costs you two geometries, not the whole
review.
Before that, one tape failing ended the target before `review-sheet` ever ran,
which meant no sheets at all, including for the nine tapes that had recorded.

It retries a failed tape once, after a pause. `vhs` loses its `ttyd` socket
often enough (`use of closed network connection`, a different tape each run,
each of them recording perfectly on its own) that a 12-tape run rarely
finished. That is a flake in the harness, not in the scene, and this is a local
review tool rather than a CI gate, so a retry is the right size of fix.

A tape that fails twice leaves **no** directory behind — including one that
recorded fine and only failed the frame check — so its stale frames from an
earlier run are never tiled into this run's sheet and read as current. Record
it by hand afterwards (`sh docs/tui-review/record-tape.sh
docs/tui-review/<name>.tape`, which files the stills where plain `vhs` does
not) and re-run `make review-sheet`.

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

Tapes copy `fixtures/xdg/` to `out/tui-review/xdg/` and point `XDG_CONFIG_HOME`
there,
so a developer's real bookmarks never appear in a frame *and* the stills that
write to the bookmarks file (`bookmark.png`, `catalog-off.png`) cannot dirty
the tracked fixture. The fixture ships one bookmark (`jonathan@tilde.team`) so
the BOOKMARKS section is on screen and the catalog stays visible; each
recording starts from that same state.

Both tours therefore open with a `Sleep 2s` before their first `Type`. ttyd's
shell is cold on a tape's first line and a keystroke that lands before the
prompt is dropped — which would skip the reset and record whatever the last
run left in `out/tui-review/xdg/`. A completed tour ends with the six-bookmark
fixture in place, so the symptom is a *second* consecutive run drifting off the
tape: `bookmark.png`'s `b` toggles a pin off instead of on, and its
`Wait+Screen /bookmarked/` then times out two minutes later per geometry.

## Sizes and themes

Every size/theme is recorded twice — once for chrome (`chrome-*.tape`, offline)
and once for responses (`responses-*.tape`, loopback fingerd). Both families
share the same six geometries. The sizes are not peers: record all of them,
then judge each by its tier. An enhancement must reproduce on a first-class
geometry. Diagnostic frames bound a first-class finding; they do not create
one.

| Size | Cells | Tier | Why |
|---|---|---|---|
| `*-80-dark.tape` | 80×24 | First-class | classic terminal; the default design target |
| `*-100-dark.tape` | 100×30 | First-class | startpage note column is designed at 100 columns |
| `*-80-light.tape` | 80×24 | First-class | AdaptiveColor on a light terminal background |
| `*-60-dark.tape` | 60×20 | Breakpoint | below `startWideMinWidth` (72); the stacked layout |
| `*-100-tall.tape` | 100×50 | Diagnostic | the whole startpage in one frame — crowded vs truncated |
| `*-45-dark.tape` | 45×24 | Diagnostic | the narrow floor: a tmux split, a side-by-side pane |

**First-class** is what lookit is designed for (macOS terminals, typically
80×24 and up, plus the 100-column note layout and light theme). Composition
findings here can become issues. **Breakpoint** is a layout the product
actually ships, so the broken rubric still applies — clip, collide, dishonest,
illegible. Making 60×20 as nice as 80×24 does not. **Diagnostic** isolates one
variable so the first-class screens can be judged at all. At 24 or 30 rows the
startpage shows only its top dozen entries, so "is this crowded" cannot be
answered — 50 rows fits BOOKMARKS, COMMUNITIES and every SERVICES group at
once. `45×24` keeps the 80-column tapes' height so width is the only thing
that changed; 60 columns is *below* the stacked-layout threshold but is not
actually narrow, and 45 is where a two-column layout runs out of room. Do not
add features whose only job is to fill 50-row surplus or comfort 45 columns.

Do not open an enhancement whose only reproducing first-class geometry is
empty. Write it on the review if you want a record; do not put it on the
board.

### Calibrating a new size

Pixel sizes are calibrated at `FontSize 18` and `Padding 0` (VHS `Set Width`
is pixels, not columns), and they are **measured, not derived** — the implied
cell height is not consistent between sizes, so arithmetic from an existing
tape gets it wrong. Record a throwaway tape that runs
`echo GEOM $(tput cols) x $(tput lines)`, screenshot it, read the numbers off
the PNG, and iterate. The current sizes measure exactly:

| Pixels | Cells |
|---|---|
| 520×580 | 45×24 |
| 686×490 | 60×20 |
| 914×580 | 80×24 |
| 1132×714 | 100×30 |
| 1132×1180 | 100×50 |

**Both pixel dimensions must be even.** ffmpeg's encoder rejects an odd width
or height, and VHS reports it only as `Conversion failed!` with no still
written.

Guessing rather than measuring is exactly how the first version of this kit
shipped every geometry two to three rows short of its label: what the tapes
called 80×24, 100×30 and 60×20 were really 80×22, 100×27 and 60×18. Nothing
failed — every frame was simply more cramped than the documented size, which
is the worst possible error in a kit whose job is judging density.

## Scenes

`tour.tape` is the chrome walkthrough. It never fetches.

| File | State |
|---|---|
| `start-input.png` | startpage, target input focused |
| `start-list.png` | startpage, first row selected (bookmark) |
| `start-child.png` | SERVICES child `dict` selected; note and `├` visible |
| `start-bottom.png` | `G` to the last row — how the catalog ends, and the last page |
| `start-nomatch.png` | `/zzzzzz` mid-type: a filter matching nothing |
| `start-filter.png` | startpage flattened by `/plan` |
| `help.png` | `?` panel on the startpage |
| `about.png` | About (`a` from the open help panel) |
| `bookmark.png` | `b` on `@cosmic.voyage`: flash, BOOKMARKS 2, catalog count drops |
| `catalog-off.png` | `catalog off` in the file: BOOKMARKS is the whole page |
| `start-many-bookmarks.png` | six pins (`fixtures/bookmarks-many.sh`): BOOKMARKS carrying real weight, every last-visited bucket, longest ago at the top |
| `start-many-filtered.png` | the same six pins under `/s`: each pinned note stands back down to its catalog description |

The last four mutate the throwaway bookmarks file, so they run last — every
earlier still shows the one-bookmark fixture state. `start-many-bookmarks`
rewrites the file wholesale rather than appending, so it also clears the
`catalog off` line the scene before it added.

### The dated fixture is generated, not tracked

A pinned row's note column holds its last-visited date as a relative phrase, so
a committed date rots: a fixture written today saying `visited 5 days
ago` says `visited over 1 year ago` next summer, and the still quietly stops
showing what the scene claims. `fixtures/bookmarks-many.sh` prints the file
with the stamps computed at record time, and the tape redirects it into the
throwaway config tree. No `Wait` in either scene matches a date — the pins land
via the overview's `YOURS 6` count — so the text staying dynamic costs the tape
nothing.

Its six pins cover every bucket `relativeDay` can produce — today, yesterday,
`N days ago`, `N months ago`, `over 1 year ago` — plus one undated pin, which
lands last so its blank note reads as *not visited yet* rather than as a feature
that failed to render. That is also what separates the six rows from each other:
before the dates existed every pinned note was blank, and
`start-many-bookmarks` could not answer its own question (shelf, or fourth
catalog section?) because the rows carried nothing the catalog does not.

The dates also order the shelf: BOOKMARKS renders longest-ago first, undated
rows last, so the still shows the fixture's six pins in the reverse of the
order the script writes them. Check that inversion when you review the frame —
it is the only place the ordering is visible, and a shelf that came out in file
order means the sort did not run. A file carrying `sort manual` keeps its own
order instead; no scene records that, because it draws the same six rows in the
same two columns and only the sequence differs — nothing this kit can judge that
`TestBuildSectionsSortManualKeepsFileOrder` does not already settle.

`start-many-filtered` is the other half. Flattening restores every row's
catalog note, so the pinned rows swap `visited …` back for their descriptions —
the one frame where the note column's two registers appear as one exchange. The
query is `s` because bubbles ranks filter matches by score, not by section: a
longer query pushes the pins below the fold, and `s` keeps a dated pin in the
first few rows even in the two-line stacked layout at 60×20. Editing
`tui/catalog.txt` reshuffles that ranking.

Two things bite in that tail and are already handled. Quitting needs `Ctrl+C`,
not `q`: lookit launches with the target input focused, so `q` types a literal
`q` into the target — harmless as a tape's last line, which is why it went
unnoticed, but fatal once another scene follows. And lookit needs a full
second to exit before the next shell line types, or the command lands in the
target input instead.

**Editing `tui/catalog.txt` can break this tape.** Three waits quote a catalog
note verbatim, because the note is the only on-screen proof that the intended
row is selected: `Look up a word` (`dict@bbs.airandwave.net`, for
`start-child`), `lucky dip` (`textfile@typed-hole.org`, for `start-bottom`) and
`Collaborative science fiction` (`@cosmic.voyage`, before the `b` press). A
reworded note leaves a wait that can never match, and VHS answers that with a
two-minute timeout per geometry and no explanation. Adding or removing a
catalog line also moves rows: `start-child` counts eight `Down`s to the first
service child, and `start-bottom` assumes `textfile` sorts last.

`responses-tour.tape` fingers the loopback fixture (`:2479` named, `:2480` generic), then a closed port.

| File | State |
|---|---|
| `list.png` | host listing at `@127.0.0.1:2479` (alice, bob) |
| `list-help.png` | `?` over a host listing — help's third context |
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

## Rubric: does it look wrong

Two rubrics live here and they answer different questions. This one asks
whether a screen is *broken* — collisions, clipping, illegible pairings,
dishonest copy. The aesthetic rubric below asks whether a screen that renders
correctly is any *good*. Run this one first; a composition question about a
frame that is already colliding is wasted effort.


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
   note column at 100 columns and crush at 60, a pinned row's `visited …` date
   against the catalog prose in the same column, BOOKMARKS vs catalog hierarchy,
   `├`/`└` children, selection shelf, gradient wordmark, input placeholder
   (`user@host or @host`) vs the list below it.

## Rubric: is it any good

A separate pass, run after the one above, and deliberately subjective. Nothing
here can be a test — that is the point. Work these five lenses in order, and
apply them to the startpage first: it is the launch screen, it is the densest
thing lookit draws, and it is where the catalog's growth shows up first.

**1. Density and breathing room.** What is the ink-to-space ratio? Does any
section run more than about seven rows without a break? Where two mechanisms
add vertical space — a section separator *and* an unconditional spacer — do
they ever compound into a gap that reads as a mistake? Does the note column
crowd the target column at 100, and which of the two gives way first at 45?
`chrome-100-tall` is the density evidence for the first-class page;
`chrome-45-dark` is diagnostic — it tells you what breaks, not what to build.

**2. Hierarchy and scanning.** Two-second test: opening `start-input.png`
cold, is it obvious where to start? Do the section headers carry more weight
than the rows, or compete with them? Does BOOKMARKS read as *the user's own
shelf*, or as a fourth catalog section that happens to be on top?
`start-many-bookmarks.png` is where this is decidable — one seeded bookmark
cannot pose the question.

**3. Alignment and rhythm.** Do column edges hold across sections, or does
each section set its own? Are the `├`/`└` connectors aligned to their parent
and to each other? Does the note column's left edge stay put as the target
column's contents change width? Does the 48-cell note cap produce truncation
that reads as deliberate or as ragged?

**4. Restraint.** Count what is on one screen: distinct colours, distinct
weights, distinct glyph classes, distinct alignment rules. Then ask of each
whether it is earning its place. Is the overview line above the list carrying
information or repeating the status bar? This lens is the one that most often
finds something worth removing, which is the cheapest kind of fix.

**5. Intuitiveness at rest.** The true first-run screen is the input focused
with nothing selected. Is the next action obvious without the help panel? Does
the placeholder compete with the list beneath it for the eye? And at the edges
— the last page, an empty filter result — does lookit look considered or
merely finished? `start-bottom.png` and `start-nomatch.png` exist for this.

Then run lenses 1–4 again on **help**, which is now responsive: it retains the
longest prefix of its candidate set that fits, so what it drops between 100 and
45 columns is a design decision no test asserts. Compare `help.png`,
`list-help.png` and `reader-help.png` on the first-class tapes. A
bottom-docked overlay that only looks stranded on `responses-100-tall` is a
diagnostic note, not a reason to invent a new help geometry.

Repeat lenses 1–4 once more, briefly, on the reader and the status chrome.

Findings are worth more when they carry the frame that shows them. Rank by
severity times first-class reproduction, not times how many of the six tapes
show it. A diagnostic or breakpoint frame can confirm or bound a first-class
finding; it cannot promote a note into an issue. Something wrong only at 45
or only as "60×20 would be nicer with a new interaction" stays a note.

## Contact sheets

`make review-tui` finishes by writing one `out/tui-review/<tape>-sheet.png` per
directory: every still of that tape, half size, tiled four across. `make
review-sheet` alone rebuilds them from frames already on disk. Cells are in
filename order and the target prints the manifest for each sheet as it goes.

The sheet is an index, not the review — read it to find the frames worth
opening at full size, then open those. Twelve sheets is a practical first pass
where the individual stills are not. Triage in this order: `chrome-80-dark`,
`chrome-100-dark`, `chrome-80-light` (and their `responses-*` pairs); then
`chrome-100-tall` for density context; then `chrome-60-dark` and
`chrome-45-dark` for the broken rubric only.

## Agent review

1. `make review-tui` (or record the tape that matches the change).
2. Read the contact sheets in the order above to triage, then open the
   full-size PNGs that look wrong. They are images; there is no GIF to review.
3. Work the two rubrics against the scene list above. Apply the size tiers
   when deciding whether a finding is an issue: first-class can create
   enhancements; breakpoint only if the stacked layout is broken; diagnostic
   never creates an enhancement on its own.
4. Visit every first-class size/theme that shares the chrome you touched.
   Record 60 and 45 as well when the change is in the stacked layout or in
   wrapping/clipping — those tapes are how breaks there get caught. Do not
   file work whose only beneficiary is one of those sizes.
5. Leave `View()` tests and contrast tests as the regression net.

Do not drive `./lookit` from this harness expecting a TTY — `tea.NewProgram`
needs a real terminal, which VHS provides and the agent sandbox does not.
