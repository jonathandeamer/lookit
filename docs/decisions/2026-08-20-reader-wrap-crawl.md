# Reader wrapping: Crossed Fingers crawl evidence

Date: 2026-08-20
Status: record

## Context

[Issue #177](https://github.com/jonathandeamer/lookit/issues/177) requested a
`w` toggle that would reflow `.plan` prose. Lookit's renderer deliberately
preserves response-body line layout because Finger bodies are unstructured and
can contain ASCII art, tables, menus and other preformatted text. Before
changing that contract, the question was whether overlong prose occurs in the
public Finger content a lookit user can actually reach.

Crossed Fingers publishes an index of remote Finger addresses at
`@crossed-fingers.andros.dev`. This record preserves the point-in-time method
and measurements used to evaluate the request. It does not make the directory
or its changing remote bodies a repository fixture.

## Method

The crawl used lookit's own `finger.ParseTarget` and `finger.Query` so every
body passed through the same connection lifecycle, CRLF normalisation, size
cap and sanitisation as the application.

1. Query `@crossed-fingers.andros.dev` and parse each non-empty, whitespace-free
   response line containing `@` as a target.
2. Query all parsed targets sequentially, with no concurrency and a one-second
   delay between target requests.
3. Give each target a 12-second context deadline. Count a request as an empty
   failure only when it returns both an error and no body; analyse a partial
   body when one is available.
4. Split each sanitised body on its physical newlines and measure each line
   with `ansi.StringWidth`.
5. Record accounts with at least one line wider than 80 terminal cells.

For triage only, a wide line was called prose-like when it contained at least
eight whitespace-separated fields and at least 40 Unicode letters. This was a
simple review aid, not a product classifier and not a proposed heuristic for
lookit. An account was prose-like if any of its wide lines met that test.

No response bodies were retained or copied into the repository.

## Results

The directory listed 135 addresses. Of those requests, 129 completed or
returned a partial body that could be analysed; six failed without a body.
Twenty of the 129 analysed results contained at least one physical line wider
than 80 cells. Nineteen met the prose-like review heuristic, while one was a
formatted challenge list.

| Target | Maximum line width | Lines over 80 | Review class |
|---|---:|---:|---|
| `smog3@typed-hole.org` | 702 | 56 | prose-like |
| `smog7@typed-hole.org` | 678 | 57 | prose-like |
| `smog5@typed-hole.org` | 665 | 55 | prose-like |
| `smog4@typed-hole.org` | 657 | 45 | prose-like |
| `smog2@typed-hole.org` | 620 | 64 | prose-like |
| `smog8@typed-hole.org` | 567 | 26 | prose-like |
| `smog6@typed-hole.org` | 561 | 48 | prose-like |
| `smog1@typed-hole.org` | 478 | 35 | prose-like |
| `charlie_root@annihilation.social` | 176 | 1 | prose-like |
| `chrkrhc@omg.lol` | 154 | 1 | prose-like |
| `dcc@logografos.com` | 117 | 1 | prose-like |
| `david@collantes.us` | 111 | 10 | prose-like |
| `david@netbros.com` | 111 | 10 | prose-like |
| `pyrate@pyratebeard.net` | 111 | 5 | prose-like |
| `root@pyratebeard.net` | 111 | 5 | prose-like |
| `ben@text.thebenmeadows.com` | 103 | 2 | prose-like |
| `wheresalice@envs.net` | 93 | 1 | prose-like |
| `smog@typed-hole.org` | 91 | 1 | prose-like |
| `tomasino@cosmic.voyage` | 81 | 3 | prose-like |
| `epoch@thebackupbox.net` | 102 | 1 | formatted/non-prose |

Several rows are mirrors or related service endpoints, so the table is not a
claim of twenty unique authors or twenty independent content shapes. The
directory and remote bodies are mutable, and the six empty failures mean this
was not a complete observation of every listed target's content.

## Decision

The crawl supports an optional reader mode: genuinely impractical
500–700-cell prose lines exist, as do more ordinary 100–175-cell prose lines.
It does not support wrapping by default. Most observed bodies had no line over
80 cells, and the formatted counterexample confirms that width alone cannot
identify prose safely.

The reader therefore keeps its original unwrapped default and offers explicit
per-response word wrapping. Offline visual-review prose is original text
modeled on the measured line shapes above; it does not embed live `.plan`
content.
