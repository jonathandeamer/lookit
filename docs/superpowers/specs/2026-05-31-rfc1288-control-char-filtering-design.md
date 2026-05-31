# RFC 1288 §3.3 control-character filtering design

## Goal

Satisfy RFC 1288 §3.3 — *"By default, this program SHOULD filter any
unprintable data"* — the one client-side SHOULD lookit currently does not meet.
A hostile or garbled finger response can today emit raw escape sequences
(clear-screen, set-title `ESC]0;…`, cursor moves, OSC-8 hyperlinks, BEL spam,
spoofed prompts) straight into the user's terminal, because the response body
flows essentially verbatim from the socket to the screen.

We close this by **defanging control data at ingress** — once, in
`finger.Query`, the single narrow waist every terminal-writing path flows
through — and **visualizing rather than deleting** the offending bytes, so no
information is lost and lookit's "show me the real bytes" ethic is preserved.

## Why ingress, not the render path

`render.Render` is *not* the only path body-derived bytes take to the terminal.
Filtering there would leave holes and create a standing obligation to remember
the filter at every future display site:

| Path to terminal | Through `render.Render`? | Carries body bytes? |
|---|---|---|
| CLI one-shot | yes | yes (body) |
| TUI reader viewport | yes | yes (body) |
| **TUI list delegate** (`userItem.Description()` = name parsed from body) | **no** — `bubbles/list` renders it | **yes** (a hostile `Name:` reaches the screen) |
| `r` raw view | maybe | yes (cached body) |
| status breadcrumb / `y`-copy OSC 52 | no | regex-constrained host/login only |

`finger.Query`'s return value is the unique point upstream of *all* of these:
both the CLI and the TUI branch only after it returns. Filtering there makes the
guarantee hold **by construction** — complete, and robust to new display code.

Three further reasons this is the right home, not merely the convenient one:

1. **Precedent in that exact function.** `queryWith` already normalizes wire
   bytes for everyone downstream (`CRLF→LF`, `client.go:88`) and already reports
   post-normalization `meta.Bytes`. Defanging control data is the same category
   of "make wire bytes safe and uniform before anyone sees them."
2. **§3.3 is a requirement on the *client program*, and `finger/` is our
   client.** The package doc says "implements an RFC 1288 finger client"; the
   §3.3 filter is a client duty, not a rendering nicety. `render/` stays a pure
   formatter and needs **zero** security changes.
3. **One responsibility, well-bounded.** The sanitizer is a pure
   `[]byte → []byte` function with no I/O, unit-testable in isolation.

### Accepted costs

- `finger/` takes on a sanitize responsibility beyond raw transport. Mitigated:
  it is the same shape as the CRLF normalization already living there, and it is
  an RFC *client* obligation, so it belongs in the client package.
- The `r` raw view and `y`-copy now show / copy *defanged* bytes, not literal
  originals. This is arguably more correct: "raw" must still mean "safe to
  print," and a live ESC does not belong on the clipboard. The bytes remain
  fully legible (visualized, not deleted), so nothing is actually lost.

## The sanitizer

A pure function in `finger/`:

```go
func sanitize(body []byte) []byte
```

called in `queryWith` **immediately after** the `CRLF→LF` `bytes.ReplaceAll`
and **before** `meta.Bytes` is computed and the truncation logic runs (so byte
count and the "last byte is `\n`?" truncation check both see the final,
user-visible form, consistent with how `meta.Bytes` already means
post-normalization length).

It walks the body **rune by rune** (`utf8.DecodeRune`), classifying by Unicode
code point — not by raw byte — which is what lets us neutralize C1 controls even
when they are validly UTF-8-encoded:

- **Keep verbatim:**
  - `\t` (U+0009) and `\n` (U+000A) — the layout whitespace finger relies on.
  - Any printable rune U+0020 and above that is **not** in the C1 range, i.e.
    all normal ASCII text *and* all valid multibyte UTF-8 (café, box-drawing,
    emoji). This is the deliberate, recorded departure from §3.3's literal
    "7-bit" wording: on a UTF-8 terminal the 7-bit rule would delete every
    multibyte sequence and gut the "modern terminal" mission, while the genuine
    security-relevant set is the control ranges below.
- **Defang to caret notation** (`^X`, pure ASCII, two visible chars):
  - C0 controls U+0000–U+001F **except** tab and newline → `^@`…`^_`
    (ESC → `^[`, BEL → `^G`, a stray mid-line CR → `^M`).
  - DEL U+007F → `^?`.
- **Defang to lowercase hex** `\xXX` (pure ASCII):
  - C1 controls U+0080–U+009F, whether they arrived as a raw high byte or as a
    valid 2-byte UTF-8 sequence (e.g. U+009B CSI → `\x9b`). Caret notation only
    naturally covers C0/DEL; hex is the clear, unambiguous choice for the high
    range.
  - Any **invalid** UTF-8 byte (`utf8.RuneError` with width 1) → `\xXX` of that
    byte. This also satisfies §2.2's ASCII expectation defensively.

Caret/hex visualization (vs. deletion) is what makes RFC §3.3's "consider two
user options to view control/international characters" moot: nothing is hidden,
so there is no lossy state a toggle would need to reverse. International data is
never touched in the first place; control data is shown defanged. (See "No
toggle.")

## What does not change

- **`render/`** — untouched; its input is already safe. The `ascii-art` golden
  stays byte-identical because that capture contains **zero** ESC/control bytes
  (verified) — it is all printable text and passes through `sanitize` verbatim.
  The "ASCII art preserved verbatim" guarantee holds.
- **`ParseUsers` and the full golden corpus** — `loginRe` tokens, header cues,
  and the tab / 2-space column gaps the parsers key on are all printable and
  preserved, so list detection is unaffected and the corpus stays green.
- **`meta.Truncated` logic** — still correct: it inspects the last byte for
  `\n`, which `sanitize` never alters (newline is kept verbatim), and the
  body-cap check is unchanged.
- **History, drill, port-79 pinning, honesty flags, exit codes** — all
  unaffected.

## No toggle

RFC §3.3's second clause is *"Two separate user options SHOULD be **considered**
to modify this behavior"* — a deliberation prompt, not "SHOULD provide." We
consider it and decline:

- We **visualize** control data rather than delete it, so there is no lost
  information a "show raw" toggle would recover.
- We **never filter international/UTF-8 data**, so there is no "show
  international" need.

The only residual want a toggle could serve — rendering a *trusted* host's
intentional ANSI colour in colour — is a niche convenience, not a safety escape
hatch, and is explicitly out of scope here. The decline is recorded in
`docs/rfc1288-conformance.md` so it reads as a decision, not a gap.

This also keeps clear of the existing `r` "raw view" key, which is an orthogonal
*view* affordance (the rendered/parsed view ↔ the unrendered source body),
unrelated to control-character safety — and which, after ingress sanitization,
shows the same already-defanged bytes everything else does. No keybinding or CLI
flag is added.

## Testing

Consistent with the project's offline, injected-fakes, no-TTY discipline.

**`finger` (new `sanitize` unit tests, table-driven):**
- ESC and representative C0 controls → caret (`\x1b`→`^[`, `\x07`→`^G`).
- DEL `\x7f` → `^?`.
- Tab and newline preserved verbatim.
- Valid multibyte UTF-8 preserved verbatim (`café`, a box-drawing line, an
  emoji).
- Raw high C1 byte `\x9b` → `\x9b` (hex).
- **Validly UTF-8-encoded** C1 U+0085 (bytes `0xc2 0x85`) → `\x85` (proves
  rune-level, not byte-level, classification).
- Invalid UTF-8 byte → `\xXX`.
- **No-op on a clean body** (a normal `Login:/Name:/Plan:` profile is returned
  byte-identical) — guards against over-filtering.

**`finger` (integration, existing local-listener harness):**
- A server whose response embeds an ESC sequence yields a body with the sequence
  defanged; `meta.Bytes` equals the defanged length; `meta.Truncated` is still
  false when the body ends in `\n`.

**`render` (regression):** the existing golden corpus stays byte-identical
(no `-update` needed) — proves the render path is genuinely unchanged.

**`tui` (the concrete hole this closes):** a host response whose parsed `Name:`
field contains an ESC sequence renders, in the list delegate, with the sequence
defanged.

## Architecture summary

```
finger.Query → queryWith (client.go)
  read body → CRLF→LF  (existing)
            → sanitize(body)          ← NEW: pure []byte→[]byte, rune-walk
            → meta.Bytes / truncation (existing, now sees safe bytes)
  return (body, meta, err)
        │
        ├── main.runOneShot → render.Render → stdout      (unchanged; safe input)
        └── tui (reader / list delegate / r view / y-copy)(unchanged; safe input)
```

`finger/` → `render/` → `tui/` dependency direction unchanged. No new imports
beyond the standard library (`unicode/utf8`). No CLI surface, keybinding, or
rendering-path change.

## CI gates

Unchanged: `go vet ./...`, `gofmt -l .` empty, `golangci-lint run ./...`,
`go test ./... -race`, plus `make vuln`. The feature adds no I/O; all tests stay
offline.
