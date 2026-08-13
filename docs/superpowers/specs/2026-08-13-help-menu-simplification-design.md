# Help Menu Simplification Design

**Date:** 2026-08-13

## Goal

Make the expanded `?` help panel easier to scan without removing any keyboard
behavior. Advertise the primary gesture for link traversal, omit the advanced
top/bottom shortcut, and use the shorter, familiar name for the startpage.

## Scope

Change only the expanded help presentation:

- omit `g/G top/bottom` everywhere the expanded panel is assembled;
- show `tab next link` instead of `tab/n next link`;
- show `shift+tab previous link` instead of
  `shift+tab/N previous link`; and
- show `h home` instead of `h startpage`.

All runtime bindings remain unchanged:

- `g` and `G` still jump to the top and bottom through the active Bubble Tea
  component;
- both `tab` and `n` still move to the next detected link;
- both `shift+tab` and `N` still move to the previous detected link; and
- `h` still opens the startpage.

Status-bar hints and every other help label stay as they are. Historical design
documents and implementation plans remain point-in-time records and are not
rewritten. The current user-facing message catalogue is updated because it
documents live copy rather than historical intent.

## Design

Keep a single `key.Binding` for each action. Its `Keys()` continue to hold every
accepted runtime key, while `WithHelp` names only the primary gesture intended
for display. This uses the existing separation between matching keys and help
metadata and avoids parallel display-only bindings or renderer-specific string
rewrites.

`Jump` remains in `keyMap` and remains enabled by `updateKeymap`, but
`keyMap.FullHelp()` no longer includes it. Because all ordinary expanded help
surfaces start with `FullHelp()`, this removes the row consistently from the
startpage, reader, raw view, and user-list contexts. The links panel already
uses its own smaller group and does not include `Jump`.

## Testing

Tests must verify presentation separately from behavior:

- the expanded help omits `top/bottom` and the `Jump` binding;
- link help displays only `tab` and `shift+tab`;
- `h` displays as `home`; and
- the underlying bindings still contain `g`, `G`, `tab`, `n`, `shift+tab`,
  `N`, and `h` as appropriate.

This separation makes an accidental removal of a working alias fail even when
the simplified menu copy is correct.
