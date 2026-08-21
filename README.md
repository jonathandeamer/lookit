# ☞ lookit

A finger client built for exploring, not just querying.

<p align="center">
  <a href="https://github.com/jonathandeamer/lookit/releases"><img src="https://img.shields.io/github/release/jonathandeamer/lookit.svg" alt="Latest Release"></a>
  <a href="https://github.com/jonathandeamer/lookit/actions/workflows/ci.yml"><img src="https://github.com/jonathandeamer/lookit/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
</p>

<p align="center">
  <img src="demo/demo.gif" alt="Animated demo of lookit browsing finger servers: filtering tilde.team's user list, opening a user's .plan, and fingering another host for a weather report." width="600">
</p>

Finger is one of the oldest things on the internet ([RFC 1288](https://www.rfc-editor.org/rfc/rfc1288), TCP port 79): ask a server about a person and it tells you who they are, whether they're logged in, and whatever they've left in their `.plan` file.

It faded once the web arrived, but never died. On the small internet, [tilde communities](https://tildeverse.org/) and [pubnixes](https://tilde.wiki/Public_Access_Unix_System) still run finger servers, and people keep a `.plan` as a slow, personal microblog. lookit is for poking around that world.

## What lookit does

While the `finger` command is built around querying a specific address, lookit is built for exploring when you don't know where you're going.

It doesn't show you anything `finger` couldn't — `finger @host` has always listed who's around. What it adds is movement: the user list is selectable, links inside a response are drillable, and you can walk back through where you've been without re-fetching. It turns a string of one-off commands into one session.

It's built on the [Charm](https://charm.sh) stack, so it behaves like the TUIs you already use and adapts to light or dark terminals.

## Install

```bash
brew install --cask jonathandeamer/tap/lookit
```

Or take a `.deb`, `.rpm`, or archive for your platform from the [Releases page](https://github.com/jonathandeamer/lookit/releases) — no Go needed, which is the point on a tilde or pubnix box where the system Go is usually too old. The packages install the manual for you; from an archive, put both files where they belong:

```bash
tar xzf lookit_*.tar.gz
mkdir -p ~/.local/bin ~/.local/share/man/man1
mv lookit ~/.local/bin/
mv man/lookit.1 ~/.local/share/man/man1/
```

With Go 1.25 or newer (the version in `go.mod`):

```bash
go install github.com/jonathandeamer/lookit@latest
```

That copies the binary only, with no manual. Run `man lookit` after a package install or the archive installation above; from an unpacked archive or clone, read the page in place with `man ./man/lookit.1`. `lookit(1)` is the full reference.

## Usage

```bash
lookit                       # open the browser
lookit jonathan@tilde.team   # open it on one person
lookit @plan.cat             # open it on a host, then browse its users
lookit @tilde.team:79        # spell out the port (79 is the default)
lookit --version
```

Type a target and press Enter to fetch it. Finger a bare `@host` and, when it answers with a list of users, lookit opens that list: move with the arrows, `/` to filter, Enter to finger whoever's highlighted. Enter on a user drills in, Esc walks back through where you've been, and `r` refreshes the current response or retries a failed lookup. While a request is loading, Esc cancels it. Ctrl+C always quits.

Responses keep the server's original line layout by default; in the reader, press `w` to word-wrap long prose for the current response, and press it again to restore the original layout.

lookit opens on a startpage: a built-in catalog of finger communities and services, with your own bookmarks pinned above it. Press `↓` to browse it, `↵` to go, `h` to come back from anywhere, and `b` to bookmark whatever you're looking at — on a user list that bookmarks the host, so drill into someone before pressing `b` to bookmark the person.

Your bookmarks are a plain text file at `~/.config/lookit/bookmarks` (or `$XDG_CONFIG_HOME/lookit/bookmarks`), and it's yours to edit. lookit keeps your comments and ordering, notes the date you were last somewhere, and lists whatever you've been neglecting at the top. Put `catalog off` on its own line to hide the built-in catalog, or `sort manual` to keep the file's order instead. It re-reads the file every time you return to the startpage, so you can edit it while lookit is running. The file explains its own format in a header comment, and `lookit(1)` is the full reference.

Everything is keyboard-driven. Press `?` inside lookit for the full, context-aware key list.

## What lookit is not

- A finger server. It won't host your `.plan` or answer anyone's queries; that job belongs to `fingerd`. lookit only reads.
- A way to write: it doesn't post or edit.
- A background process. Nothing polls and nothing runs as a daemon.
- A general small-web browser. It speaks finger and follows `finger://` links, but won't fetch gopher, gemini, or the web.

## Coming soon
- Richer styling and link discovery, tuned to how today's finger servers format their menus and links.
- Discovery and subscriptions: following a `.plan` to see what's changed since you last looked.
- Maybe a local mode: finger the machine you're already on, reading its users and `.plan` files straight off disk with no network round-trip.

## Built with

lookit is built with [Charm](https://charm.sh) tools: [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss); the demo gif above was recorded with another Charm tool, [VHS](https://github.com/charmbracelet/vhs). It speaks [RFC 1288](https://www.rfc-editor.org/rfc/rfc1288) finger over TCP/79.

## Contributing

Bug reports, ideas, and PRs are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) © 2026 Jonathan Deamer.
