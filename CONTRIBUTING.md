# Contributing

lookit is a young hobby project, and contributions, bug reports, and ideas are all welcome. If you point it at a finger server it reads wrong, that's a genuinely useful report: send the response, since the parser is tested against a corpus of real captures. AI-assisted code is welcome too, as long as you've read and tested it yourself. The commit's yours, so skip the AI co-author trailers.

To build it and run the same checks CI does:

```
make build
make check    # vet, gofmt, lint, race tests
make hooks    # installs the commit-message hook (once per clone)
```

Commits follow Conventional Commits (`fix(tui): ...`, `docs: ...`), and the hook checks the subject line.

Open an issue for bugs or ideas, and for anything bigger start one before a PR so we can sort out the approach. Security issues go through `SECURITY.md` rather than a public issue. The reasoning behind past decisions lives in `docs/superpowers/specs/` and `CLAUDE.md`.
