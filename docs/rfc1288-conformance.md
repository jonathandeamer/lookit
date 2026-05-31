# RFC 1288 conformance

This records how `lookit` relates to every normative requirement in
[RFC 1288](https://datatracker.ietf.org/doc/html/rfc1288) (the Finger User
Information Protocol), and — where a requirement is not met — *why*.

The framing that governs the whole table: **lookit is a finger *client*.** It
opens a TCP connection, sends one query line, reads the response, and closes.
RFC 1288 is overwhelmingly a specification for the **RUIP** (the server, the
"Remote User Information Program") that *answers*. Of its ~20 normative
requirements, all but two bind the answering host, not the querying client, and
are therefore **out of scope by construction** — lookit is not, and does not
contain, an RUIP.

Legend: ✅ met · ➖ out of scope (server/admin requirement) · 🔶 deliberately
deferred (a client *could* do this; we chose not to, with reason).

## MUSTs

| § | Requirement (verbatim, abridged) | Applies to | Status |
|---|---|---|---|
| 2.2 | "Any data transferred **MUST** be in ASCII format, with no parity, and with lines ending in CRLF." | client (the query we send) | ✅ The query is written as `<user>\r\n` (one line, CRLF-terminated, `client.go`). "No parity" is a serial-line anachronism, N/A on TCP. Real finger usernames are ASCII; defensively, the response side now also hex-escapes any non-ASCII/invalid bytes (see §3.3). |
| 2.2 | (same sentence) | server output | ➖ |
| 2.3 | "An RUIP **MUST** accept the entire Finger query specification." | server | ➖ |
| 2.4 | "An RUIP **MUST** either provide or actively refuse this forwarding service" (and the forwarding behaviour, if provided). | server | ➖ |
| 2.5.1 | "An RUIP **MUST** either answer or actively refuse … **MUST** provide at least the user's full name." | server | ➖ |
| 2.5.2 | "An answer **MUST** include at least the full name … same amount of info as {C} **MUST** also be returned." | server | ➖ |
| 2.5.3 | "Allowable 'names' … **MUST** include 'user names' or 'login names' as defined by the system." | server | ➖ |
| 3.2.1 | "If RUIP processing of {Q2} is turned off, the RUIP **MUST** return a service refusal message of some sort." | server | ➖ |
| 3.2.2 | "If RUIP processing of {C} is turned off, the RUIP **MUST** return a service refusal message of some sort." | server | ➖ |
| 3.2.5 | "The RUIP **MUST NOT** allow system security to be compromised by that program." | server | ➖ |

**Result:** the single MUST that binds a finger client (§2.2 CRLF query) is met.
No client-side MUST is unsatisfied.

## SHOULDs

| § | Requirement (verbatim, abridged) | Applies to | Status |
|---|---|---|---|
| 3.3 | "By default, this program **SHOULD filter** any unprintable data, leaving only printable 7-bit characters (ASCII 32–126), tabs (ASCII 9), and CRLFs." | **client** | ✅ Met by ingress-time control-character filtering — see "§3.3 resolution" below. We intentionally depart from the literal "7-bit" wording to preserve UTF-8 (documented there). |
| 3.3 | "Two separate user options **SHOULD be considered** to modify this behavior" (view control / international chars). | client | ✅ *Considered, declined* — see "§3.3 resolution → No toggle". The verb is "be considered," and our visualize-not-delete approach makes a toggle unnecessary. |
| 3.1 | "An RUIP **SHOULD** protect itself against malformed inputs." | server, but spirit applies | ✅ The client applies the same defense: 1 MiB body cap, connect/read deadlines, context-cancel closing the connection, reset-after-body handled, and now control-char sanitization (`client.go`). |
| 2.5.1 / 2.5.2 / 2.5.3 | Admin **SHOULD** be allowed to include/return/choose info atoms and ambiguity behaviour. | server/admin | ➖ |
| 3.2.1 / 3.2.2 / 3.2.3 | Admin **SHOULD** be able to toggle {Q2}/{C} and tailor returned atoms; {Q2} default **RECOMMENDED** off. | server/admin | ➖ |
| 3.2.7 | "Implementations **SHOULD** allow system administrators to log Finger queries." | server | ➖ |

**Result:** both client-side SHOULDs in §3.3 are resolved — one implemented, one
considered-and-declined with reason. Remaining SHOULDs are server/admin duties
or are already honored in spirit (§3.1).

## MAYs and other client-capable options — deliberately deferred

These are things a finger *client* could legitimately do. We have considered
each and **deliberately deferred** it; none is a gap, and each has a recorded
reason. They are listed so the choice is explicit rather than accidental.

| § | Option | Status | Reason |
|---|---|---|---|
| 2.5.4 | Send the **`/W` verbose token** (`/W user`) to request fuller output. | 🔶 deferred | Niche; few live servers honor it, and lookit has no verbosity control yet. The natural home is a future "verbose" toggle that prepends `/W `. No current UX demand. |
| 2.4 | Emit **`{Q2}` host-to-host forwarding** queries (`user@host1@host2`). | 🔶 deferred | The RFC **RECOMMENDS** servers default {Q2} *off* (§3.2.1) and warns against gateways, so almost nothing answers a forwarded query. `ParseTarget` splits on the first `@`, so the chained form is not constructed today. Defensible to omit; revisit only if real demand appears. (Minor rough edge: the chained form currently fails at dial rather than with a tailored message.) |
| 2.5.2 | "There **MAY** be a way for the user to run a program in response to a Finger query." | ➖ out of scope | This is a *server* feature (and one the RFC itself flags as dangerous, §3.2.5). A client has nothing to implement. |
| 3.3 | Provide a **toggle** to view control / international characters. | 🔶 declined | See §3.3 resolution → No toggle. Visualize-not-delete makes it unnecessary; recorded as a considered decision. |

## §3.3 resolution: control-character filtering at ingress

Design: `docs/superpowers/specs/2026-05-31-rfc1288-control-char-filtering-design.md`.

**What we do.** `finger.Query` sanitizes the response body once, at ingress (the
single narrow waist every terminal-writing path flows through — the CLI and the
TUI both branch only *after* `Query` returns). The body is walked rune by rune:

- **Kept verbatim:** tab, newline, and every printable rune — including all
  valid multibyte UTF-8 (accents, box-drawing, emoji).
- **Defanged (visualized, not deleted):** C0 controls except tab/newline and
  DEL → caret notation (`ESC`→`^[`, `BEL`→`^G`); C1 controls (U+0080–U+009F,
  even when validly UTF-8-encoded) and any invalid UTF-8 byte → `\xXX` hex.

**Two recorded departures / decisions:**

1. **We decline the literal "7-bit" wording.** Stripping everything outside
   ASCII 32–126 would delete every UTF-8 sequence and gut lookit's reason to
   exist ("a finger client for the *modern* terminal"). The 7-bit rule is a 1991
   artifact; the genuinely security-relevant set is the C0/C1/DEL control ranges
   plus ESC, which we *do* neutralize. This meets §3.3's intent (no control data
   reaches the terminal live) while keeping modern content faithful.

2. **No toggle (§3.3 clause 2, "SHOULD be considered").** Because we *visualize*
   rather than delete, nothing is hidden and there is no lossy state for a "show
   raw" option to recover; because we never touch UTF-8, there is no "show
   international" need. The only residual want — rendering a trusted host's
   intentional ANSI colour *in colour* — is a convenience, not a safety control,
   and is out of scope. (It is also distinct from the existing TUI `r` key, which
   toggles the rendered view ↔ the unrendered source body, not control-char
   safety — and which shows the same defanged bytes after ingress sanitization.)

**What this closes.** Without it, a hostile or garbled response could inject
clear-screen, set-title (`ESC]0;…`), cursor moves, OSC-8 hyperlinks, BEL spam,
or spoofed prompts directly into the user's terminal — including via the TUI
list delegate, which renders a parsed `Name:` field *outside* the `render`
package. Filtering at ingress closes every such path by construction.
