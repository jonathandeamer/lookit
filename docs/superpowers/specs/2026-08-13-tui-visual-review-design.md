# TUI Visual Review Design

**Date:** 2026-08-13

**Status:** Approved design — implemented as `docs/tui-review/`

**Scope:** A repeatable way for a person or an agent to visually and
aesthetically review lookit's TTY chrome. No product behaviour change, no CI
gate, no replacement of existing `View()` or contrast tests.

## Problem

Agents cannot sit in a real TTY, and `tea.NewProgram` will not start
headlessly. Existing tests inject a fake fetch and assert on `View()` strings,
status copy, and WCAG-ish contrast. Those catch behaviour. They do not catch
column alignment as perceived, status-bar collision, startpage connectors, the
gradient wordmark under a Nerd Font, or light vs dark as a person sees them.

`demo/demo.tape` already rasterizes a PTY, but it is a marketing capture: window
card, live hosts, Sleep timings. Using it as a review fixture would mix those
constraints with inspection.

## Decision

Keep the hero tape in `demo/`. Put the review kit in `docs/tui-review/`, next
to `docs/wordmark-design/`.

- VHS `Screenshot` PNGs, one per landed screen, reviewed as images.
- Offline first: startpage, help, About. No live finger.
- Fixture `XDG_CONFIG_HOME` so developer bookmarks cannot leak into a frame.
- Dark 80×24, dark 100×30, dark 60×20 (narrow layout), light 80×24.
- Generated frames are gitignored. `make review-tui` records; `make check`
  does not.
- Reader, list, and error stills use a loopback fingerd
  (`docs/tui-review/fixtures/fingerd`, `127.0.0.1:2479` named / `:2480`
  generic) plus a closed-port dial. Extra stills cover a selected service
  child, view-source, focused link, links panel, `i` on a landed reader,
  `auto-detected`, and `partial (truncated)`. No product fetch seam and no
  live hosts. A Go scene book that dumps `View()` stays deferred.

## Amendments after first use

Recorded here rather than left as drift; the README is the current guide.

- **Stills are `Wait` → `Sleep` → `Screenshot`.** The design assumed `Wait`
  made a still trustworthy. It does not: VHS matches the wait against the live
  terminal but writes the PNG from a lagging frame queue, so the first
  recordings banked a blank `list.png` and a `generic.png` of the previous
  screen while exiting 0. Every capture now sleeps 1500ms between the two.
- **Recorded output is verified, not just counted.** `verify-frames.sh` fails
  a directory containing a blank or duplicated still.
- **`XDG_CONFIG_HOME` points at a throwaway copy** of the fixture, not the
  fixture itself, so stills that exercise bookmark writes are possible at all.
- **Scenes added:** `b` bookmarking and `catalog off` (chrome), the help
  overlay on a landed reader and a mid-scroll reader (responses).
- **Contact sheets** (`make review-sheet`) tile each tape into one image, so a
  review triages the sheets instead of opening every still.
- **Geometries are tiered, not peers.** First-class (`80×24` dark/light,
  `100×30`) can create enhancements. `60×20` is the stacked-layout breakpoint:
  the broken rubric still applies, "make it as nice as 80" does not.
  `100×50` and `45×24` are diagnostic: they bound a first-class finding and
  do not create an enhancement on their own. Rank by first-class reproduction,
  not by how many of the six tapes show the frame. Recorded in
  `docs/tui-review/README.md` after #101 and #102 were filed off diagnostic
  and breakpoint composition.

## Non-goals

- ASCII/txt goldens as the aesthetic test.
- Putting VHS in the merge gate.
- Driving Ghostty/`screencapture` from an agent (highest fidelity, worst
  automation; keep for suspected terminal-specific bugs only).
