# lookit

A finger client for the modern terminal.

Lookit talks RFC 1288 finger over TCP/79 and renders the response with chrome and structured field highlighting. Built with [Charm](https://charm.sh) tools.

## Status

Phase 1 (CLI MVP) — under construction. See [design spec](docs/superpowers/specs/2026-05-28-lookit-design.md).

## Install

```bash
go install github.com/jonathandeamer/lookit@latest
```

## Usage

```bash
lookit alice@plan.cat
lookit @tilde.team
lookit alice@example.com:7979
```

## License

TBD.
