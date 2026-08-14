# Living copy inventory

Date: 2026-08-14

## Goal

Bring the two living docs that describe lookit's authored copy and RFC stance
back in line with the code. They exist so a later copy change has a place to
land, and so a reader can tell what is implemented versus deferred. Both had
drifted.

## Scope

One docs change, two files. No Go, no README Install, no Homebrew, no Coming
soon, no CLI help rewrite.

- `docs/user-facing-messages.md` — full pass against current `main`.
- `docs/rfc1288-conformance.md` — the §2.4 forwarding row, and the §3.3
  sentence that still says the CLI renders a body.

## Inventory

Keep the table shape (Message / Source / Surface) and the note that source
references are starting points, not an API. Prefer function names over line
numbers.

Walk the source, not the old inventory.

- Delete or rehome rows that are false: rotating placeholders (the input is
  `user@host or @host`); the landing hint without `↓ browse`; `No response yet.`
  described as first launch (launch is the startpage; that string is the
  reader's empty viewport).
- Add the parse errors `finger.ParseTarget` actually returns beyond the four
  the old file listed (IPv6 brackets, invalid port, the one-relay limits).
- Add a startpage section for overview labels, section headers, the no-match
  line, file-level empty states, bookmark flashes, and file-problem notices.
- Refresh status-bar and help rows to `startBar` / `buildStatusBar` /
  `updateKeymap` / `keys.go`. While Help is open the bar clears its hints
  (state stays); the popover omits `?`.
- Catalog notes stay in `tui/catalog.txt` and are not duplicated here.
- Inherited bubbles text stays in its own table. `Filter: ` remains inherited
  until #118.

## RFC page

§2.4 is no longer "deferred, fails at dial." One-relay
`user@host@relay` / `@host@relay` (port only on the relay) is met.
Multi-relay, a port on the inner host, and forwarding inside `finger://` stay
deferred and are refused with named sentinel errors. The MAY section intro
must stop claiming every row is deferred.

§3.3's waist is still `finger.Query`. The consumers are the TUI paths (reader,
list delegate, view-source). `main.go` does not render a response body.

`/W` stays deferred.

## Check

Markdown only. Every new or changed row names a string that still exists in
the cited function. Every deleted row is gone from the app or rehomed.
