# finger/

Networking only, no UI deps. The repo-wide rules live in the root `CLAUDE.md`;
this file is the detail behind them, including the full untrusted-input
contract. `finger` tests spin up a local `net.Listen` server — never a real
network.

**Changes here are the one category an agent does not self-merge.** The sanitize
ingress, the `hasControl` egress guard, and port-79 pinning are security
invariants: push the branch, open the PR, leave the merge to the human.

## Parsing and querying

`ParseTarget` → a `Target{Query, HostPort, Raw}` (defaults the port to `:79`, and rejects C0/DEL and non-printing Unicode controls (Cf/Zl/Zp) in the user/host token — see below); `Query` → `(body []byte, Meta, error)`. `ParseTarget` accepts direct forms (`user@host`, `@host`, optional `:port`) and **one-relay forwarding** (`user@host@relay`, `@host@relay`) plus `finger://`/path-style addresses, building the exact wire line in `Target.Query` (read via `QueryLine()`); `Target.HostQuery()` reports whether a response routes as a host/list (empty query or a leading `@`, so a forwarded `@host@relay` lists too). Multi-relay forwarding, forwarded host ports, and forwarding inside `finger://` URLs are rejected for now (distinct sentinel errors).

`ParseTargetPinned` is the variant for server-supplied targets: it forces port 79 and refuses forwarding lifted from a response (see `tui/CLAUDE.md` on drilling).

The client caps the body at 1 MiB, applies a read timeout, normalizes CRLF→LF, and treats a connection reset *after* the body as success (marking `Meta.Truncated` only when the body was cut mid-line, i.e. no trailing newline). `Meta` carries `Addr/Bytes/Truncated/Elapsed`.

## Ingress: `Query` is the single untrusted-input chokepoint

The response body is sanitized there once (`sanitize`, sanitize.go) — control/escape bytes and non-printing Unicode format controls (Cf/Zl/Zp, e.g. RTL override) are visualized, not deleted. Sanitize at ingress, *not* at a display site, because body-derived bytes reach the terminal through more than one path: `render.Render` (CLI + TUI viewport), the `bubbles/list` delegate (a parsed `Name:` the renderer never sees), and the `v` view-source view. Filtering in `Query` makes the guarantee hold by construction for every current and future display path.

**A second, non-network ingress exists as of the startpage:** the bookmarks file (`tui/bookmarks.go`). It admits records of the form `<target> [<YYYY-MM-DD last-visited day>]`: one field means the date is unknown; a date is read as local midnight and must be spelled exactly as the write path emits it (zero-padded, round-trip checked), so the grammar stays one token rather than "any date"; each target must be valid UTF-8, must survive `ParseTarget`, and must contain no C1, Cf, Zl or Zp control. A malformed date refuses the whole record and is reported as a problem, never silently dropped — so only a validated `time.Time` is ever displayed. Descriptions and classifications come only from matching entries in the embedded catalog we author; comments and malformed trailing text are never displayed. The write path's first in-place rewrite is `updateBookmarkLine`: it splices by byte offset so comments, spacing, and ordering are preserved, a round-trip guard refuses to emit a record the parser would reject, and it runs only for already-bookmarked targets on successful landings, degrading silently on failure. It is dispatched as a `tea.Cmd` (`stampVisitCmd`), never called inline from `Update`, so a config dir on a network filesystem cannot stall a landing; tests that assert on the file therefore have to run the command the landing returns (`runLandingCmd`). A malformed date also refuses the record, and `updateBookmarkLine` now leaves a record alone when the rewrite would be byte-identical, so a second visit on the same day writes nothing at all. On first load when the bookmarks file is absent, `loadBookmarks` atomically creates it with a comment header (`bookmarkFileHeader`, comments only — never re-applied, so a user's edit to it survives) above `jonathan@tilde.team`; any existing file, including an empty one, is authoritative, so removing the line does not resurrect it. This is a narrow exception to the original no-seeded-defaults decision: it is an ordinary unclassified bookmark, not a catalog person entry.

Nothing from that file needs `sanitize`, because nothing unvalidated is ever displayed. **If a future ingress does admit free text, it must call `sanitize` itself and this note must change.**

The bookmarks file is a *config* ingress, not an untrusted-input one: it is user-owned (`0o600` under a `0o700` dir), so writing it already implies owning the home directory. It earns the ingress label because it parses external bytes — hand-edited by design, and able to hold a drilled target derived from a remote response body via `toggleBookmark`, filtered by `validateBookmarkRecordTarget` on the way in. Keep "untrusted" for the socket, so the word still means something where it matters; correspondingly, the bookmarks parser is not one of the security invariants that block a self-merge (root `CLAUDE.md` lists those as the sanitize ingress/egress and port-79 pinning).

## Egress

The symmetric egress guard is `hasControl` (rejecting, not stripping, C0/DEL and Cf/Zl/Zp in the outbound query in both `ParseTarget` and `queryWith`) so hostile targets cannot smuggle extra RFC 1288 query lines or misrepresent their displayed destination.

## Protocol stance

`docs/rfc1288-conformance.md` records how lookit relates to every normative
requirement in RFC 1288, and why any unmet one is unmet. It is a **living**
document: update it in the same change that alters what goes on the wire.
