# Body link detection & in-reader browsing design

## Goal

Detect addresses and links inside a finger response **body** — `.plan` /
profile text that lands in `stateReader` — so the user can move between them and
act: drill finger targets in-app, copy or ⌘-click everything else. Today only
recognized *host-list* responses get any link extraction (`appendHarvestedTargets`
in `tui/userlist.go`, gated behind a parseable user list); a plain profile body
that mentions `finger alice@host`, a `https://` URL, or a bare `bob@tilde.team`
renders as inert text. This closes that gap.

## Non-goals (v1)

- **No CLI / piped-output detection.** The CLI path (`render.Render` from
  `main.go`) is one-shot and non-interactive; there is no navigation surface
  there. Detection is a TUI-reader concern only.
- **No copy-able links in *list* preambles.** The list path already harvests
  finger targets; surfacing non-finger copy links in a list's preamble is a v2.
- **No auto-open of any link by lookit.** lookit never launches a browser or
  mail client. The terminal may (via OSC-8); lookit itself only drills finger
  and copies to the clipboard.
- **No new protocol clients.** lookit cannot fetch https/gemini/gopher/mastodon,
  IRC, RTMP, or arbitrary future URI schemes; those links are copy-and/or-⌘-click
  only.
- **No bare-domain promotion.** Text like `tilde.team`, `hashbang.sh`, or
  `fosshost.org` without a scheme remains plain text. That shape is common in
  prose and headings, and promoting it would be too noisy for v1.
- **No bare `@handle` (no host) promotion.** A hostless social handle is not a
  finger target and is too noisy to surface, unless a social cue word precedes
  it (then copy-only).

## Relationship to the 2026-05-29 generic-list-fallback decision

`2026-05-29-generic-list-fallback-design.md` made it an explicit non-goal to
"harvest arbitrary `user@host`-shaped tokens (e.g. email addresses) from prose."
**This spec deliberately revisits that, scoped differently and on purpose.** That
non-goal protected the *list parser* (`ParseUsers`): promoting a whole
selectable **list** from prose emails would be a bad false positive that hijacks
the whole screen. This feature does not promote a list — it annotates **inline
links in the reader**, each individually focusable, honestly labelled
`(auto)` when inferred, and biased to a cheap, reversible action (a finger query
that at worst times out). The risk profile is different, so the decision is
different. This is a recorded divergence, not drift.

## Action model (settled)

lookit can natively act on finger only. Therefore:

- **Finger** links → **Drill** in-app (existing drill path).
- **URL / Email / Social** → **Copy** to clipboard, never fetched. A link is
  *additionally* rendered as an OSC-8 terminal hyperlink **only when its `Raw`
  literally begins with a terminal-openable scheme** (`http://`, `https://`,
  `mailto:`); lookit never synthesizes one (see "OSC-8"). So an explicit
  `https://…` URL and an explicit `mailto:…` are clickable, while a cue-detected
  email (`email bob@host`, no scheme), non-openable schemes such as `gemini://`,
  `gopher://`, `ircs://`, or `rtmp://`, and any Social handle are copy-only.

## Taxonomy & classification

One ordered rule set. A token is surfaced as a `Link` when **any** signal fires;
**first match wins**:

1. **Explicit scheme (case a).**
   - `finger://` → Finger. Direct finger URLs drill. Forwarded finger URLs such
     as `finger://relay/user@host` drill only when the relay is the same pinned
     host as the current response (see "Server-supplied forwarding"); otherwise
     they are surfaced as copy-only blocked finger links.
   - `mailto:` → Copy (Email).
   - Any other `scheme://authority...` token with a non-empty authority → Copy
     (URL). This deliberately includes `http://`, `https://`, `gemini://`,
     `gopher://`, `ircs://`, `rtmp://`, and future schemes. Only `http(s)` gets
     OSC-8; the others are copy-only. A bare `scheme://` with no authority is
     **not** a link, so prose/code like `` `https://` instead of `git@` `` does
     not become focusable. Other colon-only scheme-like text stays out of scope
     except for `mailto:`; otherwise ordinary labels like `Project:` and
     `Timezone:` would become false positives.
2. **Cue word immediately preceding the token (case b).** Trust the cue:
   - `finger` → Finger. Direct targets drill; forwarded targets follow the same
     same-relay rule as `finger://`.
   - `email` / `e-mail` / `mail` / `contact` → Copy (Email)
   - `web` / `site` / `url` → Copy (URL)
   - `fedi` / `fediverse` / `mastodon` → Copy (Social)
3. **`@host` host-query form, host domain-sane** → Drill (Finger). The host must
   pass the domain-sanity gate below. This distinguishes `@tilde.team` (a real
   host query — drills) from `@alice` (a bare social handle — **not** surfaced;
   note `finger.ParseTargetPinned("@alice")` *succeeds* with host `alice:79`, so
   the gate, not the parser, is what enforces the no-bare-handle non-goal).
4. **Bare `user@host`, domain-sane (case c).** Host passes the gate below; token
   not embedded in a larger word. → `Kind==Finger` with `Target` populated, but
   **default `Action` is `Copy`** and `f` drills on demand (`Ambiguous = true`;
   indistinguishable from an email, so we detect and label it `(auto)` but do
   **not** fire a finger query unprompted — policy B, see "Bare-address action").

Not surfaced: bare domains; bare `@handle` with no host (unless rule 2 tags it
Social); Matrix/XMPP/IRC shorthand forms such as `Matrix: @alice:matrix.org`,
`T: @handle`, or `IRC: #lookit` beyond explicit URI schemes; anything failing
domain-sanity; tokens inside larger words/identifiers.

### Extraction boundaries and token order

The scanner reserves the **largest explicit token first** before applying the
shorter `@host` / `user@host` rules. This is load-bearing for forwarding:
`epoch@whois.ano@thebackupbox.net` is one forwarded finger candidate, not an
ambiguous `epoch@whois.ano` plus leftover text. Even when that forwarded
candidate is blocked or copy-only, it consumes the full span so no smaller
address submatch leaks through.

Token boundaries are conservative:

- `scheme://` URLs require at least one authority character after `//`.
- trailing sentence punctuation and enclosing delimiters are not part of `Raw`
  unless they are balanced inside the URL; e.g. `https://example.com/foo).`
  yields `https://example.com/foo`.
- backticks and quotes delimit links rather than joining them.
- `user@host` and `@host` candidates must not be part of a larger word,
  identifier, or larger `@`-containing token.

### Domain-sanity (the precision gate)

The make-or-break rule for a TUI. **IP literals are carved out first:** if
`host` is a bracketed literal — `user@[…]`, holding either an IPv4 dotted-quad
or an IPv6 address — it is accepted on literal-shape grounds and the
domain-label bullets below **do not apply** (a bracketed IPv6 such as
`user@[::1]` has no `.` and uses `:`, so it could never satisfy them). The gate
only validates that the brackets enclose a syntactically plausible IP.

Otherwise `host` is treated as a **domain name** and qualifies only when it:

- contains at least one `.`,
- has only valid host-label characters (`[A-Za-z0-9-]`, dot-separated, no
  leading/trailing hyphen per label),
- ends in a plausible TLD (2+ alphabetic chars; reject an all-numeric final
  label — a bare dotted-quad like `user@1.2.3.4` without brackets is **not**
  accepted, only the bracketed `user@[1.2.3.4]` form is).

In **both** cases the whole token must be bounded by non-word characters (so it
is not a substring of a longer identifier).

What this gate **does** filter: malformed tokens, hostless `@handle`s, and
tokens embedded in larger identifiers. What it **cannot** filter: a well-formed
email is syntactically identical to a finger address (`admin@example.com` vs
`bob@tilde.team` both pass), so the gate does **not** distinguish email from
finger — that is the deliberate bias-to-finger call (see "Bare-address action"
below), not something domain-sanity can decide. The gate is "precise" only about
*shape*, mirroring `ParseUsers`' decline-on-malformed philosophy at the token
level; it is not a content classifier.

### Bare-address action (chosen: B — copy-default, drill-on-demand)

A bare, cue-less, domain-sane `user@host` is genuinely ambiguous and common — it
is also exactly what every plain email looks like, and nothing syntactic tells
them apart. **Policy B:** rule 4 still *detects and labels* the address (so it is
focusable and copyable), but its default `Action` is **`Copy`**, and pressing
`f` fingers it on demand. This keeps an email-heavy profile from becoming a wall
of finger links and never fires a finger query at an address the user didn't
choose, while still surfacing genuine bare finger addresses for a one-key drill.

It is a deliberate, scoped refinement of the earlier "bias to finger" answer:
the *bias* still shows up (the address is detected as `Kind==Finger`, `f` is one
key, the label is `(auto)`), but the **default action** is the safe, reversible
one. Strong finger links (rules 1–2) and `@host` host queries (rule 3) are
**unaffected** — they remain `Enter`-drills, since they are not email-ambiguous.

## The `Link` type and the detector

A new **pure, dependency-free** file `tui/links.go`, mirroring
`userlist.go`/`ParseUsers` (pure parser, golden-corpus tested, lives in `tui/`):

```go
type LinkKind   int // Finger | URL | Email | Social
type LinkAction int // Drill | Copy

type Link struct {
    Kind      LinkKind
    Action    LinkAction    // default action for Enter. Drill for strong/@host finger;
                            // Copy for URL/Email/Social AND for an ambiguous bare address
                            // (Kind==Finger but Action==Copy; f drills it — policy B).
    Raw       string        // exact substring as it appears in the body
    Target    finger.Target // populated for drillable Finger links and ambiguous bare addresses
    Ambiguous bool          // case-c inference; drives the "(auto)" label and the f-to-drill affordance
    Forwarded bool          // candidate was a one-relay forwarding form
    Blocked   string        // non-empty for copy-only Finger links that must not drill
    Strong    bool          // matched an explicit scheme (rule 1) or cue word (rule 2),
                            // vs. an inferred @host/bare-address (rules 3–4). Gates list
                            // harvest together with !Target.HostQuery() and
                            // harvestableLogin(Target) (see adapter below).
}

func DetectLinks(body []byte, originHostPort string) []Link
```

`DetectLinks` runs on the **sanitized** body (sanitization already happened at
`finger.Query` ingress, so no control bytes survive). Finger candidates are
resolved through a server-target helper that preserves the `ParseTargetPinned`
port-79 invariant for direct targets, and adds the same-relay forwarding rule
below. A candidate that the helper rejects is dropped unless it is a blocked
forwarded target, in which case it is surfaced as copy-only so the full token can
be copied and smaller address submatches stay suppressed.

`originHostPort` is the `Entry.Target.HostPort` that produced the body. The
helper canonicalizes it by stripping any explicit origin port and rejoining with
`:79` before same-relay comparisons, matching the server-supplied target pin.

### Server-supplied forwarding

Server text may offer one-relay forwarding only through **the same host that
served the current response**. This supports real Finger Ring instructions while
preserving the existing safety boundary.

Accepted as drillable while viewing a response from `thebackupbox.net`:

```text
finger epoch@whois.ano@thebackupbox.net
finger://thebackupbox.net/epoch@whois.ano
```

Both dial `thebackupbox.net:79` and send `epoch@whois.ano`. The relay port is
pinned to 79, even if the body supplied another port. Inner forwarded host ports
remain rejected.

The same two tokens, viewed from any other origin, are rejected as drillable:

```text
finger epoch@whois.ano@thebackupbox.net
finger://thebackupbox.net/epoch@whois.ano
```

Those are still surfaced as copy-only blocked finger links (with no `Target` and
no `f` drill affordance), because copying is safe and useful. The status copy
should make the reason explicit, e.g. `forwarded finger · ↵ copy · relay blocked`.

This rule applies before bare-address inference. A blocked forwarded token still
consumes the full span, so `epoch@whois.ano@thebackupbox.net` never yields the
ambiguous submatch `epoch@whois.ano`.

The two regexes currently inside `appendHarvestedTargets` (`fingerURLRe`,
`fingerCommandRe`) are **superseded**: `appendHarvestedTargets` becomes a thin
adapter that calls `DetectLinks(body, originHostPort)`, filters to the
**harvestable** subset — `Kind==Finger && Strong && Blocked=="" &&
!Target.HostQuery() && harvestableLogin(Target)` — and maps to `[]User`. One
engine drives both paths. `routeFetch` passes `entry.Target.HostPort` into the
parse/list path so the same-relay decision is available without any DNS lookup
or network I/O inside the parser. The list harvester stays **behaviour-neutral for
non-forwarded forms**: today it trusts only `finger://` URLs and `finger
user@host` commands (strong contexts), and the `Strong` gate preserves that for
direct targets. The one intentional change is the forwarding fix above: a full
same-relay forwarded target is harvested as that full target instead of leaking a
shorter `user@host` submatch. The new inferred rules 3–4 (bare `@host`, bare
`user@host`) are reader-only and **must not** add rows to a parseable host list
(a prose `admin@example.com` or `@alice` in a list response would otherwise
become a spurious selectable entry).

The `!Target.HostQuery()` clause is **load-bearing, not belt-and-braces**: a
strong finger link can itself be a *host* query — `finger://tilde.team` (rule 1)
or a cued `finger @tilde.team` (rule 2) both produce `Kind==Finger && Strong`
with an empty user query. The superseded regexes never emitted those (both
structurally require a login: `fingerURLRe` needs a `/login` path segment,
`fingerCommandRe` needs `login@host`), so mapping every strong finger link to a
`[]User` row would *add* a spurious, unusable host-query entry and break
behaviour-neutrality. Filtering to a non-empty login query
(`!Target.HostQuery()`; equivalently, `Target.Query` is non-empty and does not
start with `@`) is what restores it.

`harvestableLogin(Target)` is the **second** load-bearing clause, for a subtler
reason: the server-target helper resolves candidates using the same permissive
target shapes as `ParseTargetPinned`, which accepts a **strictly wider** set of
logins than the old regexes did. The
superseded regexes constrained the login to a leading `[A-Za-z0-9_]` plus up to
31 more `[A-Za-z0-9_.-]` (≤32 chars, no `~`); `ParseTargetPinned` imposes none of
that, so `finger://example.com/~bob`, or a path segment longer than 32
characters, now parses to a perfectly valid `Target` that the old regex
**rejected**. Without re-imposing the old login shape, those would become *new*
selectable rows in a parseable host-list body — non-neutral. `harvestableLogin`
re-applies exactly the old regexes' login class to the parsed `Target` so the
harvested set is unchanged. (This constraint is **adapter-only**, not in
`DetectLinks`: a `~bob` finger link is a legitimate, drillable *reader* link —
the whole point of the feature — it simply must not become a *list* row.) The
reader consumes the full `DetectLinks` result — host-query and `~`-login finger
links still drill there; the list adapter consumes only the **harvestable**
subset (strong, unblocked, user-query, *and* old-login-shape), plus the explicit
same-relay forwarding correction. This keeps the existing non-forwarding
`userlist_test.go` corpus green by construction and makes the forwarding delta
deliberate rather than accidental.

## Reader UX

Detected links are computed **once per landed body** and cached on the
`histNode` (no re-scan per keypress or re-render).

### Primary: tab-cycle highlight

- `Tab` / `Shift+Tab` (and vim `n` / `N`) move a focus highlight link→link in
  the viewport; `viewport.SetYOffset` keeps the focused link on screen. `Tab`,
  `n`, `N`, `f`, and `L` are all currently unbound in the reader keymap (verified).
- `Enter` performs the focused link's **default `Action`**: Drill for a
  strong/`@host` Finger link; Copy for URL/Email/Social **and for an ambiguous
  bare address** (policy B).
- `f` **drills** the focused link when it carries a finger `Target` whose default
  action is Copy — i.e. the ambiguous case-c address. It is shown/enabled only
  for those links (for a link `Enter` already drills, `f` is hidden).
- `y` copies the focused link's `Raw` regardless of kind (a universal copy).
- Status bar while focused, by link type. The `⌘-click opens` hint appears
  **only when the link is actually emitted as OSC-8** — i.e. its `Raw` carries an
  openable scheme (see "OSC-8") — never for copy-only links, so the bar never
  promises an action that does not exist:
  - strong finger / `@host`: `link 2/4 · finger · ↵ drill · y copy · ⇥ next`
  - ambiguous bare address: `link 2/4 · address (auto) · ↵ copy · f finger · ⇥ next`
  - scheme URL / `mailto:` (OSC-8): `link 2/4 · url · ↵ copy · ⌘-click opens · ⇥ next`
  - non-openable scheme URL / cue-email / social (copy-only):
    `link 2/4 · url · ↵ copy · ⇥ next`
  - blocked forwarded finger:
    `link 2/4 · forwarded finger · ↵ copy · relay blocked · ⇥ next`
- The focus highlight uses `lipgloss.AdaptiveColor` (light/dark pair), not a
  hardcoded dark hex.

### Secondary: links panel, on `L`

- `L` (shift-L) toggles a panel built on the existing `bubbles/v2/list`
  (`listModel`) over the same cached `[]Link`. Kinds shown as
  `finger / url / email? / social`; `email?` carries the ambiguity. `Enter`
  performs the row's default `Action` and `f` drills an ambiguous address — the
  same action model as the inline view; `Esc` / `L` returns to text.
- **`L`, not `l`:** lowercase `l` is already the reader's page-right key
  (`keys.go` `Page` binds `left/right/h/l/pgup/pgdown`). A content-focused `l`
  panel toggle would steal page-right, so the panel uses `L` and lowercase `l`
  keeps paging.
- Reuses the list component and delegate wholesale; decouples navigation from
  body rendering for link-dense profiles.

### Keybindings

`Tab`/`n`/`N`/`f`/`y`/`L` are registered as `key.Binding`s in the reader keymap
and surfaced through the bubbles `help` panel — no hand-rolled help. They are
live only when content is focused (input blurred), consistent with the existing
honest-keybinding rule (so they still type into a target when the input is
focused). They are chosen to avoid the existing reader bindings (`l` page-right,
`h` page-left, `j/k` move). The `f` (finger-on-demand) binding is shown only
while an ambiguous bare-address link is focused.

## OSC-8 terminal hyperlinks

The wrap rule is **scheme-presence, not kind**: a link is rendered as an OSC-8
hyperlink (via lipgloss v2's `Style.Hyperlink(url, "id=…")`) **iff its `Raw`
literally begins with `http://`, `https://`, or `mailto:`** — the schemes a
terminal can open. The OSC-8 target is the `Raw` token verbatim; **lookit never
synthesizes a scheme it didn't see in the body.** On iTerm2 / Ghostty /
Terminal.app this makes those links ⌘-clickable — **the terminal opens them,
lookit does not** — so the "never auto-open" invariant holds while macOS users
get native click-to-open. Precedent: crush uses exactly this
(`linkStyle.Hyperlink(url, "id=hyper")`). Each link gets a stable per-link `id=`
so a wrapped link underlines as one unit on hover.

This keys off `Raw`, not `Kind`, on purpose — it matches the product's
copy-first, honest-copy convention (the about screen, lookit's only other
URL-bearing surface, renders its URLs as plain text + a `y` copy and emits no
OSC-8; copy is the universal action here too). Making OSC-8 mean *"the author
wrote something openable"* keeps it from manufacturing an action the body never
contained. Consequences:

- An explicit `mailto:bob@host` (rule 1) **is** OSC-8; a **cue-detected** email
  `email bob@host` (rule 2, no scheme in `Raw`) is **copy-only** — the cue tells
  us it's an address to copy, not that the author offered a clickable link.
- **All other explicit `scheme://` URLs** are copy-only despite being
  explicit-scheme (rule 1) links: no mainstream macOS terminal reliably opens
  `gemini://`, `gopher://`, `ircs://`, `rtmp://`, or arbitrary future schemes,
  so wrapping them as OSC-8 would again promise an action that does not exist.
  The openable set is deliberately just `http(s)://` and `mailto:`.
- **Social** links are copy-only: a `fedi`/`mastodon` handle has no scheme and no
  canonical URL without resolving the instance, so there is nothing to hand the
  terminal. (A social link written as a full `https://…` URL is Kind URL, has a
  scheme, and is clickable — consistent with the rule.)
- **Finger** links (strong, `@host`, and ambiguous bare addresses alike) are
  copy-or-drill only: no terminal `finger://` handler exists, and in-app drilling
  is the point.

Graceful on terminals without OSC-8 support: the sequences are zero-width and
simply render as plain styled text.

## Rendering integration (the main implementation risk)

`render/` transforms the body (chrome + field highlighting) and is lipgloss
**v1**, shared with the CLI — it has no concept of a "focused link" and must not
gain one. So link styling lives entirely in a **tui-side overlay pass** over the
already-rendered viewport string:

1. `render.RenderWithBackground` produces the viewport text as today.
2. A tui-side pass (lipgloss v2) locates each link's **literal `Raw` substring**
   **within the rendered *body* segment only** — *not* the header chrome — and
   wraps that span with: the focus-highlight style (if it is the focused link)
   and/or the OSC-8 hyperlink (if `Raw` begins with `http(s)://` or `mailto:`).

**Skip the header chrome.** `RenderWithBackground` writes `renderHeader(...)`
*before* the body (`render.go:27` then `:36`), and the header echoes the queried
target. If a profile body also mentions that target (e.g. you fingered
`bob@tilde.team` and the `.plan` repeats it), a naive whole-string scan would
match the **header** occurrence first and scroll focus to the chrome instead of
the body. The overlay must therefore confine matching to the body region. Two
options for the plan: (a) have `render` expose the header's rendered length / a
body-start marker so the overlay can offset past it, or (b) render header and
body separately in the reader and only overlay the body. (a) is lighter and
keeps the single `render` entry point; (b) is more robust. Decide in the plan.

Re-locating the literal substring (rather than threading byte offsets through
`render/`) keeps the two packages decoupled. Remaining risk: a link substring
appearing more than once *within the body*, or split across a wrap boundary.
Proposed handling — match in document order against the detected link sequence,
and detect on pre-wrap text so spans align with how the viewport wraps. This is
the piece most likely to need iteration.

## Security

Low new surface, by design. Two outward actions, both user-initiated:

- **Clipboard copy via OSC-52** (`tea.SetClipboard`, already used by the about
  screen and reader `y`). The body is pre-sanitized, so no control bytes reach
  the clipboard or the highlight.
- **A finger network fetch to a body-supplied host** when the user presses
  `Enter` / `f` on a detected finger link (`finger://`, `finger user@host`,
  `@host`, an ambiguous bare address, or same-relay forwarding). This is the
  genuinely **new** surface: it *expands the drill path* from list entries
  (already server-supplied via the harvest path) to **reader-body links**,
  including the inferred rules 3–4. It is still bounded by the existing
  invariants — never automatic (policy B never fires unprompted; everything else
  needs an explicit keypress), server-supplied ports are pinned to 79, forwarding
  is allowed only through the current response's pinned host, and the body is
  pre-sanitized — but "the only outward action is clipboard copy" would
  understate it: a body-supplied host is contacted on a keypress. The threat is
  the usual finger one (a connection to an attacker-named host on a user action),
  unchanged in kind from the list-drill path, only wider in reach. Same-relay
  forwarding is narrower than arbitrary server forwarding: lookit still dials
  only the server it is already displaying, and any onward RFC 1288 forwarding is
  that server's action.
- **No auto-open of non-finger links.** OSC-8 delegates opening to the terminal
  on explicit user click; lookit never spawns a process and never fetches
  http/mail itself.

Because the finger-harvest path touches the server-supplied-target invariant
(even while reusing the established pattern), the implementing PR is pushed but
**merged by the human**, per the repo's ship rules.

## Testing

A golden corpus of real `.plan` / profile captures in `tui/links_test.go`,
mirroring `userlist_test.go`, covering:

- **Detect** cases: each rule 1–4; OSC-8 wrapping keyed on scheme-presence —
  an explicit `https://…` and an explicit `mailto:…` are wrapped, while
  `gemini://…`, `gopher://…`, `ircs://…`, `rtmp://…`, a cue-detected `email
  bob@host`, and a social handle are **not** (copy-only); and that a bare
  well-formed `admin@example.com` is detected as an `Ambiguous` `Kind==Finger`
  link with default `Action==Copy` (policy B: it is **not** a decline case — the
  gate filters shape, not email-vs-finger — and `f` would drill it).
- **Decline** cases: hostless `@handle`, domain-*insane* hosts (no dot,
  bad labels, numeric-only TLD), bare domains, bare `scheme://` with no
  authority, and `user@host` embedded in a larger identifier or larger
  `@`-containing token. These are the real negatives the domain-sane gate and
  extraction-boundary rules enforce.
- **Forwarding** cases: `finger user@host@same-relay` and
  `finger://same-relay/user@host` are drillable when `originHostPort` pins to the
  same relay; the same tokens from a different origin are copy-only with
  `Blocked` set and no submatched `user@host`; inner forwarded ports and
  multi-relay forms remain rejected.
- **Punctuation** cases: trailing punctuation, quotes, and backticks delimit
  complete links (`'https://example.com'` yields `https://example.com`);
  `https://example.com/foo).` excludes the closing punctuation.
- **Strong-gate** cases: a list response whose body contains a prose
  `admin@example.com` / `@alice` must **not** gain a list row (the
  `appendHarvestedTargets` adapter filters to `Kind==Finger && Strong &&
  Blocked=="" && !Target.HostQuery() && harvestableLogin(Target)`). Include the
  two negatives the filter's extra clauses exist for:
  - a **strong host-query** — `finger://tilde.team` or `finger @tilde.team` must
    **not** gain a row, even though both are `Kind==Finger && Strong`, because
    they carry no user query (`!Target.HostQuery()`);
  - a **non-harvestable login** — `finger://example.com/~bob` and a `finger
    <33+-char-login>@host` command must **not** gain a row, even though
    `ParseTargetPinned` now accepts them, because the old regexes' login class
    rejected them (`harvestableLogin`). They **do** still appear as drillable
    reader links.
- **Overlay** cases: a body that repeats the queried target highlights the
  **body** occurrence, not the header chrome.

The `appendHarvestedTargets` refactor is additionally covered by the existing
`userlist_test.go` corpus staying green for non-forwarded harvestable links
(strong, unblocked, user-query, old-login-shape), plus explicit fixtures for the
same-relay forwarding correction.

## Out of scope / deferred to v2

- Copy-able links in list preambles.
- CLI/piped link annotation.
- Bare-domain promotion.
- Matrix / XMPP / IRC shorthand expansion beyond explicit URI schemes.
- Configurable cue-word lists or per-kind enable/disable.
- A "copy all links" bulk action.
