# lookit

A finger client you can wander around in.

<!-- TODO: replace with the VHS demo before release
<p align="center">
  <img src="docs/demo.gif" alt="lookit browsing a finger server" width="600">
</p>
-->

Finger is one of the oldest things on the internet ([RFC 1288](https://www.rfc-editor.org/rfc/rfc1288), TCP port 79): ask a server about a person and it tells you who they are, whether they're logged in, and whatever they've left in their `.plan` file. It mostly faded from the corporate internet, but it never went away. On the small internet, tilde communities and hobbyist boxes still run finger servers, and people still keep a `.plan` as a kind of slow, personal microblog. lookit is for poking around that world.

## Why you'd want it

The thing lookit changes is that finger stops being a lookup and becomes something you can walk through. The classic `finger` binary needs you to already know the answer: you type `finger alice@host` for a name and host you brought with you. lookit lets you arrive at a bare `@host`, see the people who are actually there, open anyone's profile, follow `finger://` links across to other hosts, and step back through everywhere you've been without re-fetching. How far you get depends on each server: some answer with a tidy list of users, many don't, and lookit tells you which rather than guessing.

A few other things worth knowing:

- It's built on the [Charm](https://charm.sh) stack, so it behaves like the TUIs you already use and adapts to light or dark terminals.
- It's careful with untrusted output. Control and escape bytes in a response are shown, not executed, and any link a server hands back is pinned to port 79, so a hostile `.plan` can't repaint your terminal or point you at another service.
- There's nothing to configure and no daemon, and it respects `NO_COLOR`.

## Install

```bash
go install github.com/jonathandeamer/lookit@latest
```

Or clone and `go build .`. Needs Go 1.26 or newer.

Prebuilt binaries and a Homebrew tap are coming with the first tagged release.

## Usage

```bash
lookit                       # open the browser
lookit jonathan@tilde.team   # open it on one person
lookit @plan.cat             # open it on a host, then browse its users
lookit @tilde.team:79        # spell out the port (79 is the default)
lookit --version
```

Type a target and press Enter to fetch it. Finger a bare `@host` and, when it answers with a list of users, lookit opens that list: move with the arrows, `/` to filter, Enter to finger whoever's highlighted. Enter on a user drills in, Esc walks back through where you've been, and Ctrl+C quits.

Everything is keyboard-driven. Press `?` inside lookit for the full, context-aware key list.

## What lookit is not

- A finger server. It won't host your `.plan` or answer anyone's queries; that job belongs to `fingerd`. lookit only reads.
- A way to write. No posting, no editing, and it never sends the `/W` verbose query that RFC 1288 §2.5.5 calls out as privacy-sensitive.
- A background process. Nothing polls and nothing runs as a daemon.
- A general small-web browser. It speaks finger and follows `finger://` links, but won't fetch gopher, gemini, or the web.
- A multi-hop forwarder. The deprecated `user@host1@host2` chaining is ignored.

## Coming soon

- Discovery and subscriptions: finding finger hosts worth a visit, and following a `.plan` to see what's changed since you last looked.
- Richer styling and link discovery, tuned to how today's finger servers format their menus and links.
- Maybe a local mode: finger the machine you're already on, reading its users and `.plan` files straight off disk with no network round-trip.

## Built with

lookit is built with [Charm](https://charm.sh) tools: [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss). It speaks [RFC 1288](https://www.rfc-editor.org/rfc/rfc1288) finger over TCP/79.

## License

[MIT](LICENSE) © 2026 Jonathan Deamer.
