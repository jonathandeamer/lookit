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
- **No new protocol clients.** lookit cannot fetch https/gemini/gopher/mastodon;
  those links are copy-and/or-⌘-click only.
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
- **URL / Email / Social** → **Copy** to clipboard, never fetched. URL/mailto
  additionally rendered as OSC-8 terminal hyperlinks (see "OSC-8").

## Taxonomy & classification

Three input cases, one ordered rule set. A token is surfaced as a `Link` when
**any** signal fires; **first match wins**:

1. **Explicit scheme (case a).** `finger://` → Drill. `https://`, `http://`,
   `mailto:`, `gemini://`, `gopher://` → Copy (URL / Email).
2. **Cue word immediately preceding the token (case b).** Trust the cue:
   - `finger` → Drill (Finger)
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

Not surfaced: bare `@handle` with no host (unless rule 2 tags it Social);
anything failing domain-sanity; tokens inside larger words/identifiers.

### Domain-sanity (the precision gate)

The make-or-break rule for a TUI. A bare `user@host` qualifies only when `host`:

- contains at least one `.`,
- has only valid host-label characters (`[A-Za-z0-9-]`, dot-separated, no
  leading/trailing hyphen per label),
- ends in a plausible TLD (2+ alphabetic chars; reject all-numeric final label
  unless bracketed IPv4/IPv6 — IPs are accepted),
- and the whole token is bounded by non-word characters (so it is not a
  substring of a longer identifier).

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
    Target    finger.Target // populated for any Kind==Finger (incl. ambiguous), via ParseTargetPinned
    Ambiguous bool          // case-c inference; drives the "(auto)" label and the f-to-drill affordance
    Strong    bool          // matched an explicit scheme (rule 1) or cue word (rule 2),
                            // vs. an inferred @host/bare-address (rules 3–4). Gates list harvest.
}

func DetectLinks(body []byte) []Link
```

`DetectLinks` runs on the **sanitized** body (sanitization already happened at
`finger.Query` ingress, so no control bytes survive). Finger candidates are
resolved through **`finger.ParseTargetPinned`** — the port-79 pin and
server-forwarding refusal hold by construction, identical to the existing
harvest path. A candidate that `ParseTargetPinned` rejects is dropped.

The two regexes currently inside `appendHarvestedTargets` (`fingerURLRe`,
`fingerCommandRe`) are **superseded**: `appendHarvestedTargets` becomes a thin
adapter that calls `DetectLinks`, filters to **`Kind==Finger && Strong`**, and
maps to `[]User`. One engine drives both paths, but the list harvester stays
**behaviour-neutral**: today it trusts only `finger://` URLs and `finger
user@host` commands (strong contexts), and the `Strong` gate preserves exactly
that — the new inferred rules 3–4 (bare `@host`, bare `user@host`) are reader-only
and **must not** add rows to a parseable host list (a prose `admin@example.com`
or `@alice` in a list response would otherwise become a spurious selectable
entry). The reader consumes the full `DetectLinks` result; the list adapter
consumes only the strong subset. This keeps the existing `userlist_test.go`
corpus green by construction.

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
- Status bar while focused, by link type:
  - strong finger / `@host`: `link 2/4 · finger · ↵ drill · y copy · ⇥ next`
  - ambiguous bare address: `link 2/4 · address (auto) · ↵ copy · f finger · ⇥ next`
  - url/email/social: `link 2/4 · url · ↵ copy · ⌘-click opens · ⇥ next`
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

URL and `mailto:` links are additionally wrapped as OSC-8 hyperlinks via lipgloss
v2's `Style.Hyperlink(url, "id=…")`. On iTerm2 / Ghostty / Terminal.app this
makes them ⌘-clickable — **the terminal opens them, lookit does not** — so the
"never auto-open" invariant holds while macOS users get native click-to-open.
Precedent: crush uses exactly this (`linkStyle.Hyperlink(url, "id=hyper")`). Each
link gets a stable per-link `id=` so a wrapped URL underlines as one unit on
hover. Finger links are **not** OSC-8 (no terminal `finger://` handler, and
in-app drilling is the point). Graceful on terminals without OSC-8 support: the
sequences are zero-width and simply render as plain styled text.

## Rendering integration (the main implementation risk)

`render/` transforms the body (chrome + field highlighting) and is lipgloss
**v1**, shared with the CLI — it has no concept of a "focused link" and must not
gain one. So link styling lives entirely in a **tui-side overlay pass** over the
already-rendered viewport string:

1. `render.RenderWithBackground` produces the viewport text as today.
2. A tui-side pass (lipgloss v2) locates each link's **literal `Raw` substring**
   **within the rendered *body* segment only** — *not* the header chrome — and
   wraps that span with: the focus-highlight style (if it is the focused link)
   and/or the OSC-8 hyperlink (if URL/mailto).

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

Low new surface, by design:

- Only outward action is **clipboard copy via OSC-52** (`tea.SetClipboard`,
  already used by the about screen and reader `y`). The body is pre-sanitized,
  so no control bytes reach the clipboard or the highlight.
- Finger targets go through **`finger.ParseTargetPinned`** (port-79 pin,
  server-forwarding refused) — unchanged invariant.
- **No auto-open.** OSC-8 delegates opening to the terminal on explicit user
  click; lookit never spawns a process.

Because the finger-harvest path touches the server-supplied-target invariant
(even while reusing the established pattern), the implementing PR is pushed but
**merged by the human**, per the repo's ship rules.

## Testing

A golden corpus of real `.plan` / profile captures in `tui/links_test.go`,
mirroring `userlist_test.go`, covering:

- **Detect** cases: each rule 1–4, OSC-8 wrapping for URL/mailto, and that a bare
  well-formed `admin@example.com` is detected as an `Ambiguous` `Kind==Finger`
  link with default `Action==Copy` (policy B: it is **not** a decline case — the
  gate filters shape, not email-vs-finger — and `f` would drill it).
- **Decline** cases: hostless `@handle`, domain-*insane* hosts (no dot,
  bad labels, numeric-only TLD), and `user@host` embedded in a larger
  identifier. These are the real negatives the domain-sane gate enforces.
- **Strong-gate** cases: a list response whose body contains a prose
  `admin@example.com` / `@alice` must **not** gain a list row (the
  `appendHarvestedTargets` adapter filters to `Kind==Finger && Strong`).
- **Overlay** cases: a body that repeats the queried target highlights the
  **body** occurrence, not the header chrome.

The `appendHarvestedTargets` refactor is additionally covered by the existing
`userlist_test.go` corpus staying green (behaviour-neutral for the strong-signal
finger subset).

## Out of scope / deferred to v2

- Copy-able links in list preambles.
- CLI/piped link annotation.
- Configurable cue-word lists or per-kind enable/disable.
- A "copy all links" bulk action.
