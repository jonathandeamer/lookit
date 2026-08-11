# Bookmarks & catalog startpage

**Date:** 2026-08-11
**Issue:** [#43 — Feature Request: Bookmarks / Startpage](https://github.com/jonathandeamer/lookit/issues/43)
**Status:** design agreed, not yet implemented

## Problem

On launch lookit shows an empty reader containing the string `No response yet.`
(`tui/app.go`, `gotoLanding`). You can only go somewhere if you already know
where you are going — which is the opposite of what the README claims lookit is
for ("built for exploring when you don't know where you're going").

The README's *Coming soon* promises "discovery and subscriptions: finding finger
hosts worth a visit", and the v0.1.0 notes say discovery is next. Issue #43 asks
for the manual half of that: a browsable startpage of bookmarked resources.

## The two sources

Discovery and bookmarking are related but distinct, and conflating them ages
badly. The original design spec
(`2026-05-28-lookit-design.md`, Phase 3) already treated them separately — a
bundled `catalog.json` for discovery, `subscribe` for the user's own list.

| | Owner | Mutable at runtime | Purpose |
|---|---|---|---|
| **Catalog** | maintainer, embedded via `go:embed` | no — changes on release | where to start |
| **Bookmarks** | user, on disk | yes — `b`, or hand-edit | what you found |

The startpage renders both as one list, bookmarks first.

**Why not seed defaults into the user's file** (as #43 suggests): once copied,
defaults can never be improved for existing users. Everyone who ran lookit once
would be frozen on the v0.2 list, and finger hosts go dark regularly — two of
the 55 addresses surveyed for this spec were already dead. Embedding the catalog
keeps defaults updatable and keeps the user's file purely theirs.

## Data model

Both sources parse into one type:

```go
type startEntry struct {
    target string // raw, parsed with finger.ParseTarget
    kind   kind   // community | service | person
    note   string // catalog only
    source source // catalogSource | bookmarkSource
}
```

`kind` drives **display grouping only** — never routing. See
[Divergences](#divergences-from-issue-43), item 1.

## File format

One grammar, shared by both files: `<kind> <target> [note]`, where the note is
the trimmed remainder of the line. Blank lines and `#` comments are preserved.

```
# ~/.config/lookit/bookmarks

catalog off                                    # optional directive

community  @tilde.team
service    weather@bbs.airandwave.net
person     jonathan@tilde.team
```

**Location:** `$XDG_CONFIG_HOME/lookit/bookmarks`, falling back to
`~/.config/lookit/bookmarks`. Deliberately *not* `os.UserConfigDir()`, which on
macOS resolves to `~/Library/Application Support` and would bury a file meant to
be hand-edited. Crush routes around it the same way.

**The `catalog off` directive** hides the catalog entirely, for users whose own
bookmarks have outgrown it. Absent means on. There is no grammar collision: a
line's first token is either a kind or a directive, and those sets are disjoint.

**Why plain text rather than TOML or JSON.** TOML is the more idiomatic format
for a modern Go CLI, and if this file were read-only that is what we would use.
But `b` writes it, and the deciding constraint is what happens on the second
write to a file the user has hand-edited. Loading into a struct and
re-marshalling destroys every comment, blank line and grouping they wrote.
Line-oriented text makes adding an append and removing a line deletion, both
preserving the rest of the file byte-for-byte. It also avoids a new dependency
in `THIRD_PARTY_NOTICES.md` for a format modelling `<kind> <target>` pairs, and
suits a finger client: greppable, `known_hosts`-shaped, no tooling needed.

The cost is a bespoke format — no editor highlighting, and real structure later
(tags, per-entry ordering) would need a migration. Acceptable: bookmarks are
deliberately a flat list of pointers, and migrating a 20-line text file is cheap.

**Bookmark lines take exactly two fields.** The parser is shared but mode-gated
(`parseLine(line string, allowNote bool)`): the catalog permits a trailing note,
the bookmarks file does not. Trailing text in a bookmarks line makes the line
malformed — skipped, with the usual line-numbered notice.

This is stricter than it first appears it needs to be, and deliberately so. An
earlier draft honored hand-written notes as "incidental generosity", which
silently broke the ingress guarantee below: a note is free text, and displaying
it sends whatever bytes it holds through the list delegate. Free text in the
bookmarks file is therefore not accepted at all, rather than accepted and
sanitized. The user loses nothing they were promised — notes were always a
catalog feature — and the invariant holds by construction rather than by a
sanitizer we would have to remember to call.

## The screen

```
target: @plan.cat                                    ← input row, unchanged

  BOOKMARKS
  @tilde.team
  Big, friendly pubnix                               ← note borrowed from catalog

› jonathan@tilde.team
  person                                             ← no catalog match: kind fills the line

  COMMUNITIES
  @plan.cat
  Finger-first microblogging

  SERVICES
  weather@bbs.airandwave.net
  Weather and 7-day forecast — weather:city@…

──────────────────────────────────────────────────
  28 entries    ↵ go · b bookmark · / filter · ? help
```

Every row is the same two-row cell `userDelegate` already renders: **target** on
the title line, **note** on the description line.

**Notes travel with the target, not the file.** When a bookmark's target matches
a catalog entry, the startpage shows the catalog's note. Bookmarking
`@tilde.team` pins it to the top *and keeps its description*, though the
bookmarks file stores only `community @tilde.team`. A bookmark with no catalog
match shows its kind instead, which is honest and fills the cell. This is what
earns the catalog-only-notes decision: annotation without the user writing one.

**Bookmarking dedups.** A bookmarked catalog entry is suppressed from its
catalog section rather than appearing twice, so `b` reads as "pin to the top".

**Sections.** `BOOKMARKS` first, in file order, kinds mixed — then the catalog
grouped `COMMUNITIES` / `SERVICES`. A `PEOPLE` heading renders only if a future
catalog gains `person` entries; today it never appears, since the catalog ships
none and bookmarks all sit in `BOOKMARKS` regardless of kind. Filtering with `/`
flattens:
headers drop out, matches are against target and note.

## Architecture

`bubbles/v2/list` has **no native section support**, and pagination assumes a
uniform `delegate.Height()`. So sections must be constructed.

**Chosen: a `startModel` wrapping a real `list.Model`.** Section headers are
items in the slice, rendered by the delegate at the same cell height as an entry
(blank line + heading), with `CursorUp`/`CursorDown` wrapped to step over them.
Headers drop out naturally while filtering, since their `FilterValue` is empty.

This inherits `/` filtering, pagination, paginator dots and the existing
`applyListStyles`/`defaultUserDelegate` treatment, so the startpage moves exactly
like the user list people already know — which is what the repo's UI priority
(real `bubbles` components first) requires.

The cost is the cursor-skip wrapper, which is fiddly at first item, last item,
page boundaries, and `Select(0)` landing on a header. Contained in one helper
with table tests.

Rejected:

- **Plain viewport with a hand-rolled cursor** — total layout freedom, but
  re-implements cursor movement, filtering and scrolling, the exact things
  `bubbles/list` exists to provide.
- **Reuse `listModel`** — cheapest, but it carries a `finger.Target` host it
  would not have, and headings could not interleave (one preamble above
  everything, not sections).

**A fourth `appState`.** `stateStart` is used only at `pos == -1` and never
stored in a `histNode`; About already establishes a state that is not a history
entry. `gotoLanding` becomes `gotoStart`. History semantics are otherwise
untouched.

## Keys

| Key | Action | Enabled when |
|---|---|---|
| `b` | Toggle bookmark | Content focused, and a target is in scope (see below) |
| `h` | Jump to the startpage, truncating history | Content focused, not already at `pos == -1` |

**What `b` acts on**, by screen:

| Screen | Target | Effect |
|---|---|---|
| Reader | The current node's target | Add, or remove if already bookmarked |
| List | The current node's *host* target, not the highlighted user | Add or remove |
| Startpage, bookmark row | That row | Remove |
| Startpage, catalog row | That row | Add — the row moves up into `BOOKMARKS`, keeping its note |
| Startpage, empty state | — | Disabled |

Bookmarking a list screen deliberately captures the host rather than the
highlighted user: `b` on `@tilde.team` means "I want to come back to this
directory". To bookmark a person, drill into them and press `b` there.

**Kind inference.** A bookmark added with `b` records `community` when
`Target.HostQuery()` is true and `person` otherwise. This is a guess and can be
wrong — `service` in particular is not inferable, since
`weather:99501@bbs.airandwave.net` is `user@host`-shaped. That is acceptable
because `kind` only affects display, and the user can correct it by editing one
word in the file.

`h` displaces its current page-left alias in `keyMap.Page` (`tui/keys.go`);
`←/→` and `pgup`/`pgdn` keep paging, which is what the status bar advertises.
Both keys are matched only in the content-focused branch, following the rule
About already sets — so `b` still types a `b` into `bob@host`.

`b` flashes `✓ bookmarked @tilde.team` / `✓ removed @tilde.team`, reusing the
existing flash mechanism, and flashes the error on a write failure.

The startpage gets its own status bar in place of `landingBar`. `FullHelp` gains
both keys so `?` stays truthful.

**Why `h` truncates rather than pushing.** Browser-faithful Home would push a
history entry so Esc returns to where you were, but `histNode` wraps an `Entry`
(target, body, meta) and the startpage has none of those. It would need
Entry-less nodes threaded through `snapshot`/`restore`/`captureRefreshView` plus
answers for what `r`, `v` and `y` do on a response-less node — a lot of surface
on delicate machinery for a modest gain. `h` is therefore exactly equivalent to
holding Esc.

## Focus model

The input row keeps its current behavior (focused at launch, rotating
placeholder), so letter keys would otherwise type rather than act — the
collision CLAUDE.md already documents for `a`.

- **Input focused** (launch): `↓` or `Tab` moves focus into the list. `↓` is free
  in `textinput`, and this reads like a browser omnibox dropping into suggestions.
- **List focused:** letter keys act. `i` returns to the input, as everywhere else.
- **Esc:** from the list, returns focus to the input; from the input, quits.

Esc always backs out one level and eventually quits. This refines today's "Esc at
root quits" into "Esc at root quits from the input", which avoids the trap where
a user presses Esc to leave the input and lookit exits from under them.

## Failure modes

All non-fatal.

| Situation | Behavior |
|---|---|
| No file | Catalog shows. Normal first run, no message |
| Unreadable file | Catalog still shows, plus a notice naming the path |
| Malformed line | Line skipped; notice names file and line number |
| Unwritable file | `b` flashes the error; nothing else breaks |
| Two instances | Read-modify-write per `b`; last write wins |
| `catalog off`, no bookmarks | Empty state naming the cause and the fix |

The empty state reads: *"No bookmarks yet. The catalog is off — remove
`catalog off` from `<path>` to see it."*

`<path>` is the **resolved** bookmarks path, never a hardcoded
`~/.config/lookit/bookmarks`. When `$XDG_CONFIG_HOME` is set, the active file is
elsewhere, and printing the fallback would send the user to edit a file that has
no effect. The same resolved path is used in the unreadable-file and
malformed-line notices. Rendering it shortened with `~` (as Crush's
`home.Short` does) keeps it readable without making it wrong.

Malformed lines are surfaced rather than swallowed, per the repo's convention of
labelling lookit's own uncertainty plainly.

Writes are atomic (temp file + rename) at `0600`, with the directory created
`0700` on first write only. Reading never creates anything.

## Trust and the untrusted-input invariant

CLAUDE.md claims `finger.Query` is the single untrusted-input chokepoint. A
config file is a new ingress, so this needs an explicit answer.

The answer is that **the bookmarks file carries no free text at all**:

- **Kind** is validated against a closed set of three keywords.
- **Target** must survive `finger.ParseTarget`, which rejects C0/DEL via
  `hasControl`, **plus** an additional check described below.
- **Notes** are rejected outright in bookmark lines (see
  [File format](#file-format)); the only notes displayed are the catalog's,
  which we author and compile in.

Every displayed byte is therefore validated or ours, by construction.

**`hasControl` is not sufficient on its own.** It rejects ASCII C0 and DEL
(`finger/query.go`), but not the non-printing Unicode format controls — Cf/Zl/Zp,
notably U+202E RIGHT-TO-LEFT OVERRIDE — that `sanitize` visualizes in response
bodies. A target is displayed, so a bookmarks line could otherwise smuggle a bidi
override onto the screen and spoof what host you are about to finger. The
bookmarks loader therefore rejects any target containing a Unicode format
control, as a malformed line.

Rejecting matches the philosophy already applied to targets: `hasControl`
rejects rather than strips, because a target is something we send, not something
we display verbatim. Bodies are visualized; targets are refused.

**Follow-up, deliberately out of scope.** The same gap exists for targets from
*any* source — a typed target, or a `finger://` link lifted from a hostile
response, can currently contain U+202E and be rendered in the breadcrumb. That is
a pre-existing weakness in `finger/`, not one this feature introduces, and
fixing it means changing a security invariant that CLAUDE.md says needs a human
merge. It should be its own issue rather than a rider on this one.

CLAUDE.md should be updated to record the bookmarks file as a second ingress and
to note that its guarantee comes from rejecting free text, not from `sanitize`.

Targets from both files parse with plain `ParseTarget`, **not**
`ParseTargetPinned`. Both are authored at the same trust level as a typed target
(the user's own config; the maintainer's own catalog), so forwarding and explicit
ports stay allowed. Pinning exists for server-supplied targets, and neither of
these is one.

## Structure

Three new files, keeping this out of the already-large `app.go`:

- `tui/bookmarks.go` — grammar, load, atomic save, path resolution
- `tui/catalog.go` + `tui/catalog.txt` — the embedded curated list
- `tui/start.go` — `startModel`, section assembly, cursor-skip wrapper

## Testing

Follows the repo's injected-fakes convention: the bookmarks path is an unexported
package var tests stub, exactly as `main.go` stubs `startTUI`, with real file
work confined to `t.TempDir()`. No network.

- **Round-trip preservation** — add and remove against a file with comments,
  blank lines, odd spacing and a `catalog off` line; assert everything but the
  target line is untouched.
- **Free text is refused** — a bookmarks line with trailing text is malformed and
  skipped, and a target containing a Unicode format control (U+202E) is rejected.
  These two guard the ingress invariant, so they are correctness tests, not
  parser trivia.
- **Messages name the resolved path** — with `$XDG_CONFIG_HOME` set to a temp
  dir, the empty state and both notices quote that path, not the `~/.config`
  fallback.
- **Catalog guard** — every embedded entry parses with `ParseTarget`, has a valid
  kind, and has a non-empty note. Makes it impossible to ship a typo'd catalog,
  which matters because the catalog is hand-edited data.
- **Cursor skip at the edges** — first row, last row, page boundaries,
  `Select(0)` on a header. The fiddly part of the chosen architecture.
- **Section assembly** — dedup of a bookmarked catalog entry, note borrowing by
  target match, `catalog off`, the empty state.
- **App level** — `h` from depth truncates history, `b` toggles both directions,
  Enter routes through the existing `submit` path, focus transitions.

## Initial catalog

Every address below was probed live on **2026-08-11** using lookit's own
`finger.Query` and `ParseUsers`. Source list:
[640kb.neocities.org/fingerverse](https://640kb.neocities.org/fingerverse/) —
55 addresses surveyed, 53 alive.

### Communities (9)

| Target | Note | Probe |
|---|---|---|
| `@plan.cat` | Finger-first microblogging | list, 1454 users |
| `@tilde.team` | Big, friendly pubnix | list, 40 users |
| `@happynetbox.com` | Hosted .plan pages, no shell account needed | list, 25 users |
| `@telehack.com` | Retro-computing sandbox; .plan pages autogenerated | list, 47 users |
| `ring@thebackupbox.net` | The Finger Ring — a webring for finger servers | list, 20 users |
| `@cosmic.voyage` | Collaborative sci-fi fiction | list, 7 users; **slowest at 6.9s** |
| `@athena.dialup.mit.edu` | MIT Athena, still answering | list, 8 users |
| `@zaibatsu.circumlunar.space` | Circumlunar Space pubnix | list, 3 users |
| `@chunboan.zone` | Small community server | banner, no list |

### Services (18)

| Target | Note | Probe |
|---|---|---|
| `@bbs.airandwave.net` | Menu of a dozen-plus finger services | menu, 1976b |
| `weather@bbs.airandwave.net` | Weather and 7-day forecast — `weather:city@…` | usage stub |
| `@graph.no` | Weather worldwide by place name — `finger oslo@graph.no` | usage stub |
| `quake@bbs.airandwave.net` | Latest earthquakes, M2.5+ past day | live data |
| `dict@bbs.airandwave.net` | Dictionary — `dict:word@…` | usage stub |
| `urban@bbs.airandwave.net` | Urban Dictionary — `urban:word@…` | usage stub |
| `wordsearch:today@bbs.airandwave.net` | Daily word search puzzle | live |
| `sudoku:easy@bbs.airandwave.net` | Sudoku, easy mode | live |
| `textfile@typed-hole.org` | A random file from textfiles.com | live |
| `calendar@flanigan.us` | Historical calendar: on this day | live |
| `bot@happynetbox.com` | Auto news bot: article titles and URLs | live |
| `random@happynetbox.com` | Jump to a random happynetbox user | live |
| `ansi@happynetbox.com` | ANSI art over finger | 17.9 KiB |
| `browserversion@happynetbox.com` | Current browser version numbers | live |
| `1@happynetbox.com` | Interactive fiction, chained over finger | drillable |
| `cyoa@typed-hole.org` | Choose your own adventure | drillable |
| `smog@typed-hole.org` | Saturday Morning Gemzine — back issues | drillable |
| `originsfinger@happynetbox.com` | Les Earnest on the origins of finger | live |

The last four are the entries that best demonstrate lookit specifically: their
bodies chain through `finger N@host` references, which lookit already detects and
makes drillable.

### People — none

**The catalog ships no personal addresses.** People are what bookmarks are for:
you meet someone while browsing, press `b`, and they are on your startpage. The
catalog is where to start; bookmarks are who you found. The `person` kind still
exists, because bookmarks use it.

This was reconsidered once. An earlier draft included six entries under a
"finger-infrastructure authors" criterion — people who publicly build finger
software and therefore evidently want the traffic. Verifying each against its
actual finger response did not support it:

| Candidate | What the response actually says |
|---|---|
| `@fuwn.net` | *"managed using gigi, a small but mighty finger server"* → the only clear match, and even it says "managed using", not "wrote" |
| `michael@mozz.us` | Name and job title only; the smallnet-tooling attribution was recalled, not evidenced |
| `@codevoid.de` | Name only; no infrastructure evidence |
| `@sava.rocks` | Runs a finger server; does not author one |
| `grawity@nullroute.lt` | Sessions and contact links; the name attached in the draft appeared nowhere in the response |
| `jaakko@skyjake.fi` | A `.plan` last updated **2024-02-06**; the Lagrange attribution was recalled, not evidenced |

A criterion that selects one entry out of six is not a criterion. Shipping
unverified biographical claims attached to a person's address, inside a binary
other people install, is a poor place to be approximately right — so the section
is dropped rather than rewritten. Two further considerations pointed the same
way: personal servers churn fastest (both dead addresses in the survey were
personal domains), and one candidate publishes a phone number in his response,
which a catalog entry would amplify beyond the webpage it already sits on.

### Surveyed but excluded

| Address | Reason |
|---|---|
| `solderpunk@circumlunar.space` | Dead — connection refused |
| `@maze.io` | Dead — network unreachable |
| `say@happynetbox.com` | Closed to new posts; visible content is now scanner spam |
| `coke@cs.cmu.edu` | The CMU Coke machine now answers `No Plan` — a dead end. Its story belongs in the README |
| `weather:{99501,10005,96813,85019}@bbs.airandwave.net` | Four US ZIP codes for one service. Covered by `weather@…` and `@graph.no` |
| `mlb_standings@bbs.airandwave.net` | Parochial for a global audience |
| `david@netbros.com` | Byte-identical to `david@collantes.us` (2968b) — same person, two domains |
| All 20 surviving personal domains | The catalog ships no people — see [People — none](#people--none) |

**Note on probing.** An initial sweep at concurrency 6 reported eight failures,
six of them on `bbs.airandwave.net`. Re-probing serially with 6s pauses showed
all six were alive — the host rate-limits. Only two addresses are genuinely down.
Any future catalog refresh must probe serially.

## Divergences from issue #43

Recorded because the design departs from the reporter's proposal in five places,
all deliberately.

1. **Enter's behavior is response-derived, not kind-derived.** The issue says
   "pressing Enter on a community fetches its user list; pressing Enter on a
   service queries it directly." Here `kind` is display grouping only, and
   `routeEntry` decides list-vs-reader from the actual response. The outcome
   matches for well-behaved hosts, but it is not a promise — `@chunboan.zone` is
   a real community whose root returns a banner. Making `kind` a routing promise
   would have lookit assert structure the protocol does not carry, against the
   honesty convention.
2. **No current-directory config.** The issue floats "current directory or
   ~/.config/lookit/". Wishlist supports a cwd config because SSH endpoints are
   project-scoped; finger bookmarks are not. Two locations means precedence
   rules, merge semantics, and a question about which file `b` writes. Easy to
   add later.
3. **Defaults are not seeded into the user's file.** See
   [The two sources](#the-two-sources) — seeding freezes defaults for existing
   users.
4. **The catalog is hardcoded, in the letter if not the spirit.** The issue asks
   for extensibility "without hardcoding anything". The user's list is entirely
   theirs and `catalog off` disables ours, but shipped defaults live in the
   binary and change only on release.
5. **The reporter's own example is cut.** `weather:99501@bbs.airandwave.net`
   appears verbatim in #43; we ship `weather@bbs.airandwave.net` and `@graph.no`
   instead.
6. **The shipped catalog contains no people.** The issue describes a list of
   "communities, services and even specific user URLs". Users can bookmark
   anyone, so the *feature* covers it — but the defaults we ship do not. See
   [People — none](#people--none).

Items 1 and 3 are the load-bearing ones and are worth explaining on the issue.

## Out of scope

Deliberately excluded; none has a demonstrated need.

- **Per-entry hiding of catalog items.** `catalog off` covers the stated case.
- **An in-app toggle for the directive.** File-only.
- **Subscriptions and change-diffing.** Bookmarks are static pointers;
  subscriptions need stored last-seen state per target. Separate phase, as the
  original design spec had it.
- **Refreshing the catalog from a URL.** The Phase 3 spec floated it; lookit
  speaks finger and nothing else, and adding an HTTP fetch to a finger client is
  its own decision.
- **Reordering bookmarks in-app.** The file is the ordering.
- **The lipgloss v1→v2 consolidation.** `render/` remains on v1 by existing
  deliberate decision. This feature lives entirely in `tui/`, which is already
  v2, so there is no forcing function; bundling a migration whose blast radius is
  all rendered output would couple unrelated risks in one review.
