# Shared walkthrough for the chrome review tapes. No Set/Output here —
# each chrome-*.tape applies size and theme first, then Sources this file.
#
# Screenshots land in docs/tui-review/frames/ with stable names; `make review-tui`
# moves them under frames/<tape-name>/ after each recording.
#
# CAPTURE ORDER MATTERS. Every still is Wait (the state is on screen) then
# Sleep (VHS's frame pipeline catches up) then Screenshot. A Screenshot taken
# straight after a Wait writes an OLDER frame — VHS matches the wait against
# the live terminal but writes the still from a frame queue that lags behind
# when the screen has been idle. 1s was reliable in testing; 1500ms is the
# margin. Dropping these sleeps silently banks stills of the wrong screen.

Hide
# lookit writes to the bookmarks file (the `b` still below), so each recording
# gets a throwaway copy under the gitignored frames/ tree. The fixture in
# fixtures/xdg/ stays pristine and every run starts from the same state.
Type "rm -rf docs/tui-review/frames/xdg && cp -R docs/tui-review/fixtures/xdg docs/tui-review/frames/xdg"
Enter
Type "export XDG_CONFIG_HOME=$PWD/docs/tui-review/frames/xdg"
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
Screenshot docs/tui-review/frames/start-input.png
Sleep 250ms

# ↓ leaves the target input and selects the first startpage row.
Down
Sleep 1500ms
Screenshot docs/tui-review/frames/start-list.png
Sleep 250ms

# Bookmark + 6 communities + bbs parent → first service child (dict).
# Wait on the child's note, which only appears when that row is selected.
Down 8
Wait+Screen /Dictionary lookup/
Sleep 1500ms
Screenshot docs/tui-review/frames/start-child.png
Sleep 250ms

Type "/"
Sleep 200ms
Type "plan"
Sleep 1500ms
Screenshot docs/tui-review/frames/start-filter.png
Sleep 250ms

Escape
Sleep 200ms
Type "?"
Wait+Screen /about lookit/
Sleep 1500ms
Screenshot docs/tui-review/frames/help.png
Sleep 250ms

Type "a"
Wait+Screen /modern TUI browser/
Sleep 1500ms
Screenshot docs/tui-review/frames/about.png
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
Screenshot docs/tui-review/frames/bookmark.png
Sleep 250ms

# The flash clears after 2s; quit once it has gone so the next launch is clean.
Sleep 2s
Type "q"
Sleep 500ms

# `catalog off` hides the built-in catalog: BOOKMARKS is then the whole page.
Hide
Type "echo 'catalog off' >> docs/tui-review/frames/xdg/lookit/bookmarks"
Enter
Type "clear"
Enter
Show
Type "./lookit"
Enter
Wait+Screen /BOOKMARKS/
Sleep 1500ms
Screenshot docs/tui-review/frames/catalog-off.png
Sleep 250ms

Type "q"
Sleep 200ms
