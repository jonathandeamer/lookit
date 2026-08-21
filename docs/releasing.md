# Releasing

Everything needed to cut a release. `CLAUDE.md` keeps only the one-line rule
(releases are tags off green `main`); the mechanics live here so a session that
isn't releasing doesn't carry them.

## The pipeline

`.github/workflows/release.yml` runs `make release` (GoReleaser) on a pushed
`v*` tag, building macOS/Linux × amd64/arm64 archives plus a `linux/armv6`
archive, `.deb`/`.rpm` packages for each Linux target, creating a **draft**
GitHub release, and publishing the updated Homebrew cask to
`jonathandeamer/homebrew-tap` (via `HOMEBREW_TAP_GITHUB_TOKEN`). Validate the
config locally with `make release-check` and dry-run a build with
`make release-snapshot`. Workflow actions are pinned to commit SHAs (Dependabot
keeps them current).

The `linux/armv6` build is the Pi Zero / Pi 1 baseline (ARM1176JZF-S). One
ARMv6 build covers the ARMv7 boards too, so there is deliberately no separate
`goarm: 7` archive. The `.deb`/`.rpm` packages are built by nfpm from those same
binaries and attached to the release as files — they are not a repository, so
`apt install ./lookit_*.deb` works and plain `apt install lookit` does not.

The release archives bundle `README.md`, `LICENSE`, `man/lookit.1`, and a generated
`THIRD_PARTY_NOTICES.md` (the dependency license texts, required when
redistributing the compiled binaries; regenerate with `make notices` after
changing deps).

## The Homebrew tap

`jonathandeamer/homebrew-tap` is the install route the README gives macOS users
first, so treat it as part of the release rather than a convenience. A stable
tag makes GoReleaser write `Casks/lookit.rb` to the tap's `main` branch;
`skip_upload: auto` keeps prerelease tags out of it entirely, so `brew upgrade`
can never serve an untested build.

**Confirm the cask actually published before calling a release done.** The
release job talks to another repository with a token this repo can't test any
other way, and GoReleaser continues through later publishers after a cask error
before failing at the end — so a red job can still have produced a perfectly
good draft release. After the workflow finishes:

```bash
brew update && brew install --cask jonathandeamer/tap/lookit
man lookit
```

If the push failed, the generated file is in `dist/homebrew/Casks/lookit.rb`
from `make release-snapshot` and can be committed to the tap by hand; the
archives and the GitHub release are unaffected either way.
`HOMEBREW_TAP_GITHUB_TOKEN` was verified on 2026-08-21 as a classic token with
`repo` scope and `push: true` on the tap.

The tap used to serve lookit from a hand-written formula pinning the v0.1.0
source tarball and building with `depends_on "go" => :build`. That was deleted
in jonathandeamer/homebrew-tap@d0234ee, ahead of the v0.2.0 tag, so the tap
holds exactly one lookit definition and an unqualified name has nothing to
resolve ambiguously. Anyone still on the formula install needs
`brew uninstall lookit` before installing the cask.

Casks are macOS-first, and the README offers Homebrew for macOS only. Homebrew
6.0.x does install casks on Linux — 6.0.16 fixed the case of a cask with
`on_linux` blocks and no `os` stanza, which is exactly what GoReleaser generates
here — but advertising that would mean carrying a minimum Homebrew version for
no gain over the `.deb`/`.rpm`, which cover more Linux targets than the cask
does (the `armv6` build has no cask at all). `brews:`, the formula equivalent of
`homebrew_casks`, still generates valid config but is deprecated and fails
`make release-check`, so a formula is not the way back.

## Version injection

Release version info is injected via
`-ldflags "-X main.version=… -X main.builtAt=…"`, with a `debug.ReadBuildInfo()`
fallback in `main.go` so a plain `go install …@latest` (no ldflags) still shows a
real version. A module-proxy install carries no build date, so `versionString`
omits the `(built …)` suffix when the date is unknown (matching the about screen,
which hides the date row) rather than printing `built unknown`; it also carries a
leading `v` that ldflags-injected release versions don't, so `moduleVersion`
trims it and both installs print `lookit version 0.2.0`. GoReleaser performs this
injection on a tagged release (see `.goreleaser.yaml`).

Building from source needs Go 1.21+: the deps use the stdlib `cmp`/`slices`
packages, and the toolchain auto-fetches the `go 1.25.0` declared in `go.mod`, so
`go install`/`go build` work on 1.21+ but fail with
`package cmp/slices is not in GOROOT` on the older Go common to tilde/pubnix
boxes — which is exactly what the prebuilt release binaries are for.

That 1.21 floor assumes `GOTOOLCHAIN=auto` and a reachable module proxy, which is
how a person installing by hand gets it. **A distro packager gets neither**: ports
build sandboxes pin `GOTOOLCHAIN=local` and forbid network access, so for them the
`go` directive is a hard requirement with no auto-fetch escape, and the tree has to
carry a toolchain at least that new. That is why the directive states the true
floor rather than whatever is current — it is set by the Charm v2 stack
(`bubbletea/v2`, `bubbles/v2`, `lipgloss/v2`, `ultraviolet`, `colorprofile`) plus
`x/sys`/`x/sync`, all of which declare `go 1.25.0`; nothing in lookit's own code
needs it. Don't raise it by hand. `go mod tidy` will raise it on its own when a
dependency bump demands it, and that is the only reason it should move.

## Versioning

**0.x SemVer read loosely**: bump **minor** for any release carrying a `feat:`,
**patch** for fix/refactor/docs-only. The shipped artifact is a binary, not an
imported library, so there's no package API to "break"; 0.x freely changes the
CLI/UX surface and **1.0 is deferred** until that surface is stable (and would
invite Go's `/v2` module-path tax).

## Prereleases: betas and release candidates

GitHub has one **pre-release** classification; beta and RC are readiness
conventions expressed in the SemVer tag, not different GitHub release types. Use
`vX.Y.Z-beta.N` when testing may still lead to meaningful changes. Use
`vX.Y.Z-rc.N` only when the build is believed ready to become `vX.Y.Z` unchanged
unless testing finds a blocker. Either hyphenated tag matches the same `v*`
trigger and runs the identical release pipeline, so testers download real
archives instead of building from source. GoReleaser creates the GitHub release
as a draft with its pre-release flag set automatically; publishing that draft
makes it a GitHub pre-release.

Two `auto` switches in `.goreleaser.yaml` key off the hyphen and make both stages
safe: `release.prerelease` prevents the release from claiming "Latest release" or
the `/releases/latest` redirect, and `homebrew_casks[].skip_upload` keeps it out
of the tap so `brew upgrade` never serves it. Both switches are no-ops for a
plain `vX.Y.Z` stable tag — set once, never toggled per release.

Use dot-separated counters (`-beta.1`, `-rc.1`): SemVer compares numeric
identifiers numerically, so `-beta.10` sorts after `-beta.9`, whereas `-beta10`
compares as a string and sorts wrong. `go install …@latest` ignores prereleases
by itself.

**The one manual step any prerelease creates:** `changelog.use: git` diffs
against the previous *reachable* tag, so the stable release following a beta or
RC would list only the commits since the most recent prerelease. Set
`GORELEASER_PREVIOUS_TAG` to the last stable tag for the stable run — add it to
the `release` step's `env:` in `.github/workflows/release.yml` before pushing the
stable tag (revert after), or run `make release` locally with it exported — so
the notes span the whole release. The draft is the checkpoint: read the generated
notes before publishing.

Testers on macOS need one warning — a browser download carries
`com.apple.quarantine` and the unsigned binary is blocked, so have them fetch
with `curl -L` or run `xattr -d com.apple.quarantine ./lookit` (the Homebrew cask
normally handles this, and prereleases deliberately skip the cask).

To test work still on a branch, don't tag at all: releases are tags off green
`main`, and `make release-snapshot` cross-compiles all four targets locally for
hand-delivery.

## Notes

There is no hand-maintained `CHANGELOG.md`: GoReleaser generates grouped notes
from the Conventional Commit subjects into the draft. For a release that
deserves prose, `docs/release-notes/` holds hand-written notes (one file per
release, e.g. `2026-06-05-v0.1.0.md`) whose body **replaces** the auto-generated
commit changelog in the draft before publishing — each file's leading HTML
comment carries its own paste instructions.
