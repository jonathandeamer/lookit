# Releasing

Everything needed to cut a release. `CLAUDE.md` keeps only the one-line rule
(releases are tags off green `main`); the mechanics live here so a session that
isn't releasing doesn't carry them.

## The pipeline

`.github/workflows/release.yml` runs `make release` (GoReleaser) on a pushed
`v*` tag, building macOS/Linux × amd64/arm64 archives plus a `linux/armv6`
archive, `.deb`/`.rpm` packages for each Linux target, creating a **draft**
GitHub release, and opening a **draft pull request** with the updated Homebrew
cask in `jonathandeamer/homebrew-tap` (via `HOMEBREW_TAP_GITHUB_TOKEN`). The
workflow does not write to the tap's `main` branch. Validate the config locally
with `make release-check` and dry-run a build with `make release-snapshot`.
Workflow actions are pinned to commit SHAs (Dependabot keeps them current).
`HOMEBREW_TAP_GITHUB_TOKEN` must be a classic token with `repo` scope, or a
fine-grained token scoped to the tap with Contents and Pull requests set to
write.

The `linux/armv6` build is the Pi Zero / Pi 1 baseline (ARM1176JZF-S). One
ARMv6 build covers the ARMv7 boards too, so there is deliberately no separate
`goarm: 7` archive. The `.deb`/`.rpm` packages are built by nfpm from those same
binaries and attached to the release as files — they are not a repository, so
`apt install ./lookit_*.deb` works and plain `apt install lookit` does not.

The release archives bundle `README.md`, `LICENSE`, `man/lookit.1`, and a generated
`THIRD_PARTY_NOTICES.md` (the dependency license texts, required when
redistributing the compiled binaries; regenerate with `make notices` after
changing deps).

## The Homebrew tap at the next stable release

`jonathandeamer/homebrew-tap` currently serves lookit from a **hand-written
formula**, `Formula/lookit.rb`, added on 2026-08-02. It pins the v0.1.0 source
tarball and builds from source (`depends_on "go" => :build`). The cask
automation in `.goreleaser.yaml` landed after v0.1.0 was tagged and there has
been no stable tag since, so `homebrew_casks` has never actually run and
`Casks/lookit.rb` does not exist yet.

The first stable tag after v0.1.0 makes GoReleaser create a branch named for the
lookit version and open a draft tap PR containing `Casks/lookit.rb`. It does not
merge the cask or change the tap's `main` branch. Prerelease tags skip the tap
entirely.

Finish the first stable release in this order:

1. Let the release workflow finish successfully. Review the draft GitHub
   release and the draft tap PR. GoReleaser continues through later publishers
   after a cask publishing error, then fails the release job at the end. Treat a
   red workflow as a failure, and confirm the PR exists before continuing. Do
   not merge it while its archive URLs still point at an unpublished release.
2. Publish the GitHub release.
3. On the generated tap branch, delete `Formula/lookit.rb` and update the tap
   README so the same PR replaces the old formula with `Casks/lookit.rb`.
4. Check out the generated branch in the local tap before testing it:

   ```bash
   brew tap jonathandeamer/tap
   tap_repo="$(brew --repo jonathandeamer/tap)"
   git -C "$tap_repo" fetch origin lookit-0.2.0
   git -C "$tap_repo" switch --detach FETCH_HEAD
   HOMEBREW_NO_AUTO_UPDATE=1 brew info jonathandeamer/tap/lookit
   HOMEBREW_NO_AUTO_UPDATE=1 brew install --cask jonathandeamer/tap/lookit
   man lookit
   ```

   Run these on a machine without lookit installed, then run
   `git -C "$tap_repo" switch main` before merging the tap PR.
5. Tell existing v0.1.0 formula users how to remove the formula and install the
   cask in the v0.2.0 release note. Write and test that command during the
   release, when both artifacts are available.

Keeping the removal and addition in one tap PR avoids a tap with two different
`lookit` definitions and avoids a period where neither is available. Once that
PR is merged, replace this one-time checklist with the normal cask update flow.

The generated cask contains macOS and 64-bit Linux downloads, plus the man page,
so Homebrew remains an install route on both systems. The `linux/armv6` build is
available through the release archive and packages instead. GoReleaser's
formula publisher, `brews:`, is deprecated and fails `make release-check`.

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
