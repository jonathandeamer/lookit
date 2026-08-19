# Notes for the crossed-fingers.andros.dev operator

Date: 2026-08-19
Status: draft, not sent

<!--
The text under the rule is meant to be sent as-is, by email or as an issue on
whatever the service uses. Everything above it is repo bookkeeping. Findings
are from live probes on 2026-08-19; re-check before sending if that has aged.
The opt-out section is the only one tied to lookit's catalog decision rather
than to the service itself, so it is the one to cut if the conversation is
going somewhere else.
-->

---

I maintain lookit, a finger client for the terminal. It now handles
crossed-fingers.andros.dev: the account index opens as a filterable list where
Enter goes to each person's own host, and the commands on your help page are
selectable rather than something to retype.

Your server is in better shape than most finger services I have parsed against.
Line endings are CRLF throughout, with no bare LFs in any response I fetched,
which is what RFC 1288 section 2.2 asks for and what almost nothing else
manages. You also handle the /W verbose token correctly: `/W help` returns the
help page rather than treating /W as part of the query. Most custom servers
choke on that.

## Quoting on the help page

`finger "add?user@host@crossed-fingers.andros.dev"` would serve better than
`finger "add?user@host"@crossed-fingers.andros.dev`.

The shell builds identical argv for both, so nothing changes on the wire. What
the current form does is make the quotes look like part of the address rather
than shell syntax. lookit read them that way and split the address at the
closing quote, offering to finger cosmic.voyage, a host named nowhere on that
line. That was lookit's bug and it is fixed, but any client scanning response
bodies for addresses can make the same mistake.

The quotes themselves are necessary and worth keeping. `?` is a glob character,
and zsh, the macOS default shell, refuses the unquoted form outright with "no
matches found".

## An unrecognised query returns the whole index

`remove?x@y` gives a clean "Unknown command: remove". A bare token that is not a
command does not: `remove`, `commands`, `about`, and any typo all fall through
to the full 496-line listing. A mistyped command produces 13 KB of unrelated
output with no sign that anything went wrong. The error you already return for
the `?` form would suit both cases.

RFC 1288 section 2.5.1 asks an RUIP to either answer or actively refuse.
Returning an unrelated index is neither.

## Search results

`search?e` returns 497 lines, which is close to the entire index. A count line
in the style of the root's "Registered accounts (496):" would help, as would a
cap once a search matches past some number of accounts.

Search matches hostnames as well as plan text, so `search?cat` returns every
account at plan.cat alongside the genuine content matches, and the hostname hits
bury the rest.

Results are bare addresses with no indication of why each one matched, so
finding out means fingering each in turn. A line of context per hit would be the
largest single improvement available here, and it is what a client would render
as a preview.

## Keeping the listing parseable

The index format works well as it stands: two-space indent, one bare address per
line, an unbroken run, preceded by a count. That regularity is what lets lookit
present it as a navigable list rather than a wall of text. Variable indentation,
or annotations mixed into the run, would drop it back to plain text.

Worth knowing either way: a bare user@host cannot be told apart from an email
address, so lookit treats loose ones in prose as copy-only rather than firing a
query at them. That does not apply inside a recognised listing, so your current
format is fine. finger://host/user is the unambiguous alternative if you ever
want it, at some cost in readability at 496 entries.

## An opt-out

There is no documented way for someone to leave the index. `remove?` returns
"Unknown command: remove", and help does not cover it. Since `add?` accepts any
address without checking whether the requester controls it, anyone can add
anyone, and the person added has no way back out.

That is what is keeping crossed-fingers out of lookit's built-in catalog, which
is a list of places lookit points people at. A remove command, or a documented
opt-out of any kind, would settle it.
