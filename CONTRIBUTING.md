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

## Branching & releases

The project is trunk-based: `main` is the only long-lived branch, and it's protected — every change lands through a short-lived branch and a PR, even maintainer changes, so CI (`make check`) gates `main` rather than your memory. `main` requires the `test` check to be green and up to date before merge, and force-pushes and deletions are blocked, so published history stays stable. There's no `develop` or standing release branch; cut a `release/x.y` branch only if an old minor ever needs a backport while `main` has moved on.

Releases are tags, made deliberately off green `main` — never from a feature branch. Pushing a `v*` tag triggers `release.yml`, which builds the archives and opens a **draft** GitHub release for you to publish by hand (see `CLAUDE.md` for the release tooling). So the flow is: merge the PR → confirm `main` is green → tag from `main`.
