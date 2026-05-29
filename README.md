# lookit

A finger client for the modern terminal.

```
➜ alice@plan.cat   123ms ✦
Login: alice
Name: Alice Example
Directory: /home/alice
Shell: /bin/zsh
On since Mon Mar 10 09:14 (PST) on tty1
Plan:
This is my plan for today.
- finish lookit MVP
- have a snack

1.2 KiB · 123ms
```

Lookit talks [RFC 1288](https://www.rfc-editor.org/rfc/rfc1288) finger over TCP/79 and renders the response with chrome and structured field highlighting. Built with [Charm](https://charm.sh) tools.

## Install

```bash
go install github.com/jonathandeamer/lookit@latest
```

(Or clone and `go build .`)

## Usage

```bash
lookit                       # open the TUI reader
lookit alice@plan.cat        # finger a user once
lookit @tilde.team           # finger a server once (banner + user list)
lookit alice@example.com:79  # explicit port
lookit version               # print version/build info
```

In the TUI, type a target and press Enter to fetch it. Use arrows or PageUp/PageDown to scroll the response. Press Esc or Ctrl+C to quit.

Fingering a server (`@host`) that returns a list of users opens a selectable list: use the arrows to move, `/` to filter, and Enter to finger the highlighted user. Press Esc to go back to the list, and Esc again to return to the input.

Output styling adapts to your terminal's color capabilities. When stdout is piped or `NO_COLOR` is set, lookit emits plain text — `lookit user@host | grep` works as expected.

## What it doesn't do

- It doesn't post `.plan` files or write to finger servers. Read-only.
- It doesn't send `/W` (verbose). RFC 1288 §2.5.5 calls it out as privacy-sensitive.
- It doesn't run a daemon. There is no background polling.
- It doesn't follow the deprecated `user@host1@host2` forwarding form.

## Roadmap

Phase 1 (CLI MVP), Phase 2 (TUI reader), and Phase 2.5 (server user-list drill-down — finger a `@host`, browse and select a user, including fingerverse menu formats) are done. Planned next:

- **Phase 3** — subscriptions (`lookit subscribe` + `lookit refresh` for watch-and-diff) and a curated catalog (`lookit discover`).
- **Phase 4** — polish: VHS demo gif, Homebrew tap.

Design spec: [`docs/superpowers/specs/2026-05-28-lookit-design.md`](docs/superpowers/specs/2026-05-28-lookit-design.md).

## License

TBD.
