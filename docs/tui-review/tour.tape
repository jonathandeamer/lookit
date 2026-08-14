# Shared walkthrough for the chrome review tapes. No Set/Output here —
# each chrome-*.tape applies size and theme first, then Sources this file.
#
# Screenshots land in out/tui-review/ with stable names; `make review-tui`
# moves them under out/tui-review/<tape-name>/ after each recording.
#
# CAPTURE ORDER MATTERS. Every still is Wait (the state is on screen) then
# Sleep (VHS's frame pipeline catches up) then Screenshot. A Screenshot taken
# straight after a Wait writes an OLDER frame — VHS matches the wait against
# the live terminal but writes the still from a frame queue that lags behind
# when the screen has been idle. 1s was reliable in testing; 1500ms is the
# margin. Dropping these sleeps silently banks stills of the wrong screen.

Hide
# Let the shell finish starting before typing at it. ttyd's shell is cold on
# the tape's first line, and a Type that lands before the prompt is dropped —
# which silently skips the reset below and records the *previous* run's
# bookmarks state, since a completed tour ends with the six-bookmark fixture in
# place. That failure only appears on the second consecutive run, so it reads
# as a mystery rather than as a dropped keystroke.
Sleep 2s

# lookit writes to the bookmarks file (the `b` still below), so each recording
# gets a throwaway copy under the gitignored out/ tree. The fixture in
# fixtures/xdg/ stays pristine and every run starts from the same state.
Type "rm -rf out/tui-review/xdg && cp -R docs/tui-review/fixtures/xdg out/tui-review/xdg"
Enter
Type "export XDG_CONFIG_HOME=$PWD/out/tui-review/xdg"
Enter
Type "export PS1='$ '"
Enter
Type "test -x ./lookit || go build -o lookit ."
Enter
Sleep 2s
Type "clear"
Enter
Show

Type "./lookit"
Enter
Wait+Screen /COMMUNITIES/
Sleep 1500ms
Screenshot out/tui-review/start-input.png
Sleep 250ms

# ↓ leaves the target input and selects the first startpage row.
Down
Sleep 1500ms
Screenshot out/tui-review/start-list.png
Sleep 250ms

# Bookmark + 6 communities + bbs parent → first service child (dict).
# Wait on the child's note, which only appears when that row is selected — so
# this string is dict@bbs.airandwave.net's note in tui/catalog.txt, verbatim.
# Editing that note without editing this line hangs the tape on a wait that can
# never match, which is a two-minute timeout per geometry and no explanation.
Down 8
Wait+Screen /Look up a word/
Sleep 1500ms
Screenshot out/tui-review/start-child.png
Sleep 250ms

# The far end of the page. Every other startpage still shows the top dozen
# rows; this one shows how the catalog ends, where the last group's connectors
# land, and what the list looks like scrolled off its first page. The last row
# is computed, not file order: hosts sort alphabetically, so typed-hole.org is
# the last group and its children sort to cyoa, lobsters, smog, textfile.
Type "G"
Wait+Screen /lucky dip/
Sleep 1500ms
Screenshot out/tui-review/start-bottom.png
Sleep 250ms

# A filter that matches nothing, while it is still being typed. Bubbles
# renders an empty Filtering list as the empty string rather than its
# "No entries." text (list.go's populatedView returns early while filtering),
# so this is the frame that shows what a user sees mid-keystroke when the
# query stops matching.
Type "/"
Sleep 200ms
# Six z's, not three: the filter is a fuzzy subsequence match, and "zzz" still
# matched "Daily word search puzzle". No catalog row has six.
Type "zzzzzz"
Wait+Screen /zzzzzz/
Sleep 1500ms
Screenshot out/tui-review/start-nomatch.png
Sleep 250ms

Escape
Sleep 200ms

Type "/"
Sleep 200ms
Type "plan"
Sleep 1500ms
Screenshot out/tui-review/start-filter.png
Sleep 250ms

Escape
Sleep 200ms
Type "?"
Wait+Screen /about lookit/
Sleep 1500ms
Screenshot out/tui-review/help.png
Sleep 250ms

Type "a"
Wait+Screen /modern TUI browser/
Sleep 1500ms
Screenshot out/tui-review/about.png
Sleep 250ms

Escape
Sleep 200ms

# Everything below mutates the throwaway bookmarks file, so it comes last:
# earlier stills must show the one-bookmark fixture state.
# Second Escape closes the help panel About was opened from and leaves focus
# back on the empty target input — so the first ↓ enters the list at row 1
# (the bookmark) exactly as it does at the top of this tour, and the second
# lands on @cosmic.voyage. The list's own `g` (go to start) is no use here:
# with the input focused it types a `g` into the target instead.
Escape
Sleep 200ms
Down
Sleep 200ms
Down
Wait+Screen /Collaborative science fiction/
Type "b"
Wait+Screen /bookmarked/
Sleep 1500ms
Screenshot out/tui-review/bookmark.png
Sleep 250ms

# The flash clears after 2s; quit once it has gone so the next launch is clean.
Sleep 2s
Type "q"
Sleep 500ms

# `catalog off` hides the built-in catalog: BOOKMARKS is then the whole page.
Hide
Type "echo 'catalog off' >> out/tui-review/xdg/lookit/bookmarks"
Enter
Type "clear"
Enter
Show
Type "./lookit"
Enter
Wait+Screen /BOOKMARKS/
Sleep 1500ms
Screenshot out/tui-review/catalog-off.png
Sleep 250ms

# Ctrl+C, not "q": lookit launches with the target input focused, so `q` here
# types a literal q into the target instead of quitting. Ctrl+C always quits.
Ctrl+C
Sleep 200ms

# A realistic BOOKMARKS section. The fixture's single bookmark keeps every
# other still deterministic, but it cannot pose the question this one does:
# with six pins, does BOOKMARKS still read as the user's own shelf, or as a
# fourth catalog section? Pinning also empties two rows out of COMMUNITIES and
# turns two service roots structural, and one bookmark deliberately matches no
# catalog entry, so an unclassified row's blank note column is on screen.
# This rewrites the file wholesale, so it also clears the "catalog off" line
# the previous scene appended.
#
# The 1s is lookit actually exiting. 200ms was enough when `q` ended the tape,
# but here the next line types into whatever holds the terminal — and if
# lookit still does, the cp lands in the target input and the tape fails.
Sleep 1s
Hide
Type "cp docs/tui-review/fixtures/bookmarks-many out/tui-review/xdg/lookit/bookmarks"
Enter
Type "clear"
Enter
Show
Type "./lookit"
Enter
# Wait on the overview count, not on a bookmark target. The unclassified
# someone@example.org is the sixth row, and in the stacked narrow layout six
# two-line rows do not fit a 20-row terminal — it is on page two at 60×20, so
# waiting for it there hangs the tape. The overview is the top line at every
# geometry, and "6" proves this fixture loaded rather than the one-line one.
#
# YOURS is the overview's word for the count; BOOKMARKS is the section header
# below it. Match the overview label, not the header — the header carries no
# count, so a header match would pass before the fixture had loaded.
Wait+Screen /YOURS\s+6/
Sleep 1500ms
Screenshot out/tui-review/start-many-bookmarks.png
Sleep 250ms

Ctrl+C
Sleep 200ms
