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

The two sources have different schemas and meet only during section assembly:

```go
type bookmarkFile struct {
    targets       []string
    catalogHidden bool
    problems      []parseProblem
}

type startEntry struct {
    target string // raw, parsed with finger.ParseTarget
    kind   kind   // unknown | community | service | person
    note   string // catalog only
    source source // catalogSource | bookmarkSource
}
```

Catalog records parse directly into `startEntry`. Bookmark records carry only a
target; section assembly creates a bookmark-sourced `startEntry`, borrowing the
catalog entry's kind and note only when the target matches. An uncatalogued
bookmark uses `unknown` and has no description. Catalog `kind` drives **display
grouping only** — never routing. See [Divergences](#divergences-from-issue-43),
item 1.

## File format

The user file has one bookmark target per line. Blank lines and `#` comments are
preserved.

```
# ~/.config/lookit/bookmarks

catalog off                                    # optional directive

@tilde.team
weather@bbs.airandwave.net
jonathan@tilde.team
```

The embedded catalog has its own maintainer-authored grammar:
`<kind> <target> <note>`. Keeping the grammars separate prevents catalog display
metadata from becoming a claim users must make about a protocol that does not
carry it.

**Location:** `$XDG_CONFIG_HOME/lookit/bookmarks`, falling back to
`~/.config/lookit/bookmarks`. Deliberately *not* `os.UserConfigDir()`, which on
macOS resolves to `~/Library/Application Support` and would bury a file meant to
be hand-edited. Crush routes around it the same way.

**The `catalog off` directive** hides the catalog entirely, for users whose own
bookmarks have outgrown it. Absent means on. There is no grammar collision:
`catalog off` and `catalog on` are the only two-token records; a bookmark is
exactly one target token after comments are stripped.

**Why plain text rather than TOML or JSON.** TOML is the more idiomatic format
for a modern Go CLI, and if this file were read-only that is what we would use.
But `b` writes it, and the deciding constraint is what happens on the second
write to a file the user has hand-edited. Loading into a struct and
re-marshalling destroys every comment, blank line and grouping they wrote.
Line-oriented text makes adding an append and removing a line deletion, both
preserving the rest of the file byte-for-byte. It also avoids a new dependency
in `THIRD_PARTY_NOTICES.md` for a format modelling a flat target list, and
suits a finger client: greppable, `known_hosts`-shaped, no tooling needed.

The cost is a bespoke format — no editor highlighting, and real structure later
(tags, per-entry ordering) would need a migration. Acceptable: bookmarks are
deliberately a flat list of pointers, and migrating a 20-line text file is cheap.

**Bookmark lines take exactly one field.** After stripping a trailing comment,
the whole line is the target. Any other trailing text makes the line malformed —
skipped, with the usual line-numbered notice. The catalog parser is separate and
requires all three fields.

This is stricter than it first appears it needs to be, and deliberately so. An
earlier draft honored hand-written notes as "incidental generosity", which
silently broke the ingress guarantee below: a note is free text, and displaying
it sends whatever bytes it holds through the list delegate. Free text in the
bookmarks file is therefore not accepted at all, rather than accepted and
sanitized. Comments may contain arbitrary text because they are discarded before
parsing and never displayed. The user loses nothing they were promised —
descriptions were always a catalog feature — and the invariant holds by
construction rather than by a sanitizer we would have to remember to call.

## The screen

```
target: @plan.cat                                    ← input row, unchanged

  BOOKMARKS
  @tilde.team
  Small public access unix, for teaching…            ← note borrowed from catalog

› jonathan@tilde.team
                                                     ← no catalog match: blank description

  COMMUNITIES
  @plan.cat
  Classic finger, polished for the present

  SERVICES
  weather@bbs.airandwave.net
  Current weather and a 7-day forecast — weather:city@…

──────────────────────────────────────────────────
  26 entries    ↵ go · b bookmark · / filter · ? help
```

Every row is the same two-row cell `userDelegate` already renders: **target** on
the title line, **note** on the description line.

**Descriptions travel with the target, not the file.** When a bookmark's target matches
a catalog entry, the startpage shows the catalog's note. Bookmarking
`@tilde.team` pins it to the top *and keeps its description*, though the
bookmarks file stores only `@tilde.team`. A bookmark with no catalog match leaves
the description line blank: neither its shape nor the finger protocol establishes
whether it is a person or service. This is annotation without asking the user to
invent metadata.

**Bookmarking dedups.** A bookmarked catalog entry is suppressed from its
catalog section rather than appearing twice, so `b` reads as "pin to the top".

**Sections.** `BOOKMARKS` first, in file order — then the catalog
grouped `COMMUNITIES` / `SERVICES`. A `PEOPLE` heading renders only if a future
catalog gains `person` entries; today it never appears, since the catalog ships
none and bookmarks all sit in `BOOKMARKS`. Filtering with `/`
flattens:
headers drop out, matches are against target and note.

**Credit.** After the final built-in catalog entry, a dim two-row endnote reads:

```
Catalog inspired by
https://640kb.neocities.org/fingerverse/
```

Only the URL receives the same OSC-8 hyperlink treatment as HTTP(S) links in
the reader, so supporting terminals can open it directly. The endnote is not a
finger target: selection skips it, it is excluded from the entry count, and it
drops out while filtering just like a section header. It appears only while at
least one built-in catalog row remains after section assembly, before filtering;
`catalog off` therefore removes the credit along with the catalog, while an
active filter always removes it even when catalog rows match.

## Architecture

`bubbles/v2/list` has **no native section support**, and pagination assumes a
uniform `delegate.Height()`. So sections must be constructed.

**Chosen: a `startModel` wrapping a real `list.Model`.** Section headers are
items in the slice, rendered by the delegate at the same cell height as an entry
(blank line + heading), with `CursorUp`/`CursorDown` wrapped to step over them.
The catalog credit is another two-row non-selectable item. Headers and the
credit drop out naturally while filtering, since their `FilterValue` is empty.

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

**No kind inference.** A bookmark added with `b` writes only `Target.Raw`.
`Target.HostQuery()` still helps route fetched responses, but it cannot establish
whether an address is a community, service or person and is never persisted as
if it could. A matching catalog entry may supply display metadata; an unmatched
bookmark remains deliberately unclassified.

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

The answer is that **the bookmarks file carries no displayed free text at all**:

- **Target** is the only bookmark field. It must survive `finger.ParseTarget`,
  which rejects C0/DEL via `hasControl`, **plus** the additional checks below.
- **Descriptions and kinds** do not exist in bookmark records. The only displayed
  descriptions and classifications come from matching catalog entries, which we
  author and compile in.
- **Comments** are discarded and never reach a display path.

Every displayed byte is therefore validated or ours, by construction.

**`hasControl` is not sufficient on its own.** It rejects ASCII C0 and DEL
(`finger/query.go`), but not invalid UTF-8, UTF-8-encoded C1 controls
(U+0080–U+009F), or the non-printing Unicode controls Cf/Zl/Zp — notably U+202E
RIGHT-TO-LEFT OVERRIDE — that `sanitize` visualizes in response bodies. A target
is displayed, so a bookmarks line could otherwise smuggle control data onto the
screen or spoof what host you are about to finger. The bookmarks loader rejects
all of those cases as malformed lines.

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
to note that its guarantee comes from accepting only a fully validated target,
not from `sanitize`.

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
  skipped. Targets containing invalid UTF-8, a C1 control (U+009B), or a Unicode
  format control (U+202E) are rejected. These guard the ingress invariant, so
  they are correctness tests, not parser trivia.
- **Messages name the resolved path** — with `$XDG_CONFIG_HOME` set to a temp
  dir, the empty state and both notices quote that path, not the `~/.config`
  fallback.
- **Catalog guard** — every embedded entry parses with `ParseTarget`, has a valid
  kind, and has a non-empty note. Makes it impossible to ship a typo'd catalog,
  which matters because the catalog is hand-edited data.
- **Cursor skip at the edges** — first row, last row, page boundaries,
  `Select(0)` on a header, the trailing credit, and selection resetting to the
  first match when filtering removes non-entry rows. The fiddly part of the
  chosen architecture.
- **Catalog credit** — exact two-line copy after the final catalog row, an OSC-8
  hyperlink around only the URL, no contribution to the entry count, and no
  credit under filtering or `catalog off`.
- **Section assembly** — dedup of a bookmarked catalog entry, note borrowing by
  target match, `catalog off`, the empty state.
- **App level** — `h` from depth truncates history; `b` toggles both directions
  while preserving the selected target across section moves; Enter starts the
  selected target through the existing request lifecycle; filter text owns
  command letters; focus transitions.

## Initial catalog

Every address below was probed live on **2026-08-11** using lookit's own
`finger.Query` and `ParseUsers`. Source list:
[640kb.neocities.org/fingerverse](https://640kb.neocities.org/fingerverse/) —
55 addresses surveyed, 53 alive.

### Communities (9)

Every note below is traceable to the server's own words, to the 640kb listing,
or to a conclusion the response plainly supports. The **Basis** column records
which, so a future refresh can re-check rather than re-guess.

| Target | Note | Basis | Probe |
|---|---|---|---|
| `@plan.cat` | Classic finger, polished for the present | 640kb | list, 1454 users |
| `@tilde.team` | Small public access unix, for teaching and learning | server: *"we're a small public access unix system with a goal of teaching and learning"* | list, 40 users |
| `@happynetbox.com` | Finger server of user profiles, run by Ben Brown | server: *"Happy Net Box is a finger server run by Ben Brown"*, *"25 most recently updated profiles"* | list, 25 users |
| `@telehack.com` | Live system status and users; .plan pages are autogenerated | server: `TELEHACK SYSTEM STATUS`; 640kb for the autogenerated half | list, 47 users |
| `ring@thebackupbox.net` | The finger ring — join by linking it from your response | server: *"This is the finger ring! To join the finger ring: Have your finger response contain…"* | list, 20 users |
| `@cosmic.voyage` | Collaborative science fiction; users crew ships | conclusion from *"Users currently online"* + *"Who control these ships"* | list, 7 users; **slowest at 6.9s** |
| `@athena.dialup.mit.edu` | MIT Athena dialup, still answering | conclusion: hostname + a classic Unix finger table | list, 8 users |
| `@zaibatsu.circumlunar.space` | Circumlunar Space pubnix | 640kb; server says *"Currently logged in sundogs"* | list, 3 users |
| `@chunboan.zone` | A tiny shared community on one cheap server | server: *"We are a tiny shared community on a single cheap server"* | banner, no list |

### Services (17)

| Target | Note | Basis | Probe |
|---|---|---|---|
| `@bbs.airandwave.net` | Menu of a dozen-plus finger services | server: *"Below is a menu of options"* + 14 listed services | menu, 1976b |
| `weather@bbs.airandwave.net` | Current weather and a 7-day forecast — `weather:city@…` | server: *"Check current weather and a 7-day forecast"* | usage stub |
| `@graph.no` | Weather worldwide by place name — `finger oslo@graph.no` | server: usage block, incl. that exact example | usage stub |
| `quake@bbs.airandwave.net` | Latest earthquakes, M2.5+ past day | server: `LATEST EARTHQUAKES - M2.5+ PAST DAY` | live data |
| `dict@bbs.airandwave.net` | Dictionary lookup — `dict:word@…` | server: *"Dictionary Lookup / Use: finger dict:word@…"* | usage stub |
| `urban@bbs.airandwave.net` | Slang, internet terms and memes — `urban:word@…` | server: *"Look up slang, internet terms, memes, and informal definitions"* | usage stub |
| `wordsearch:today@bbs.airandwave.net` | Daily word search puzzle | server: `DAILY WORD SEARCH FINGER` | live |
| `sudoku:easy@bbs.airandwave.net` | An easy sudoku, fresh each day | server: `DAILY SUDOKU FINGER / Easy` | live |
| `textfile@typed-hole.org` | A lucky dip into textfiles.com | server: `Random Textfile: textfiles/…`; 640kb | live |
| `calendar@flanigan.us` | Today’s date, across the years | server: dated historical entries; 640kb | live |
| `bot@happynetbox.com` | News headlines, with links for the curious | server: `news:` block of titles + URLs; 640kb | live |
| `random@happynetbox.com` | Jump to a random happynetbox user | server: *"find this person again: finger …@happynetbox.com"* | live |
| `browserversion@happynetbox.com` | The latest versions across the browser world | server: a list of browsers and versions | live |
| `1@happynetbox.com` | Interactive fiction, chained over finger | server: 3 `finger N@happynetbox.com` continuations | drillable |
| `cyoa@typed-hole.org` | Choose your own adventure | server: `CHOOSE YOUR OWN ADVENTURE`; ends *"Go on to page 0 (finger 0@typed-hole.org)"* | drillable |
| `smog@typed-hole.org` | Saturday Morning Gemzine — back issues | server: *"a weekly, independent gemzine"* + 8 issue links | drillable |
| `originsfinger@happynetbox.com` | Les Earnest tells how finger began | server: `Origins of the Finger Command`, from Les Earnest, 1990 | live |

**Three** entries — `smog`, `1@happynetbox` and `cyoa` — chain through
`finger N@host` references that lookit already detects and makes drillable, so
they demonstrate the tool specifically. (`originsfinger` does *not*: its single
`finger jeffking@happynetbox.com` is an aside inside the article, not
navigation.)

### People — none

**The catalog ships no personal addresses.** People are what bookmarks are for:
you meet someone while browsing, press `b`, and they are on your startpage. The
catalog is where to start; bookmarks are who you found. The `person` catalog
kind remains available for a future curated entry, even though this release
ships none.

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
| `ansi@happynetbox.com` | Alive, but see below — it does not render as art |
| All 20 surviving personal domains | The catalog ships no people — see [People — none](#people--none) |

**Why `ansi@happynetbox.com` was dropped.** 640kb lists it as "ANSI over
Finger", and it was in an earlier draft as "ANSI art over finger". The response
contains **no escape bytes at all**: it carries 1,365 occurrences of the literal
two-character sequence `\e`, so a client sees `\e[0;44;37m` as visible text
interleaved with block-drawing characters, not colour. lookit renders it
faithfully and therefore unattractively. Nothing here is a lookit bug — the
bytes really are printable — but a catalog entry promising art would be
inaccurate, and the entry is a poor advertisement either way.

This was caught only because probe bodies are stored **post-`sanitize`**: a real
escape byte would have appeared as `^[` (caret notation), so literal `\e` proves
what the server actually sends. Keep that in mind when refreshing.

**Note on probing.** An initial sweep at concurrency 6 reported eight failures,
six of them on `bbs.airandwave.net`. Re-probing serially with 6s pauses showed
all six were alive — the host rate-limits. Only two addresses are genuinely down.
Any future catalog refresh must probe serially.

**Note on verifying notes.** Every note was checked against the full response
body on 2026-08-11, and four did not survive: `@tilde.team` was described as a
"big, friendly pubnix" when its own banner says *small*; `@plan.cat` as
"microblogging" and `@telehack.com` as a "retro-computing sandbox" on
recollection rather than evidence; and `@happynetbox.com` claimed "no shell
account needed", which no source states. The **Basis** column exists so a
refresh re-checks rather than re-guesses.

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
