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
  (`docs/tui-review/fixtures/fingerd`, `127.0.0.1:2479`) plus a closed-port
  dial. No product fetch seam and no live hosts. A Go scene book that dumps
  `View()` stays deferred.

## Non-goals

- ASCII/txt goldens as the aesthetic test.
- Putting VHS in the merge gate.
- Driving Ghostty/`screencapture` from an agent (highest fidelity, worst
  automation; keep for suspected terminal-specific bugs only).
