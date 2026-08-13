# Shared walkthrough for the chrome review tapes. No Set/Output here —
# each chrome-*.tape applies size and theme first, then Sources this file.
#
# Screenshots land in docs/tui-review/frames/ with stable names; `make review-tui`
# moves them under frames/<tape-name>/ after each recording.
# Sleep after every Screenshot: VHS captures the *next* frame, so a trailing
# Screenshot with no following Sleep writes nothing.

Hide
Type "export XDG_CONFIG_HOME=$PWD/docs/tui-review/fixtures/xdg"
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
Screenshot docs/tui-review/frames/start-input.png
Sleep 250ms

# ↓ leaves the target input and selects the first startpage row.
Down
Sleep 250ms
Screenshot docs/tui-review/frames/start-list.png
Sleep 250ms

# Bookmark + 6 communities + bbs parent → first service child (dict).
# Wait on the child's note, which only appears when that row is selected.
Down 8
Wait+Screen /Dictionary lookup/
Screenshot docs/tui-review/frames/start-child.png
Sleep 250ms

Type "/"
Sleep 200ms
Type "plan"
Sleep 400ms
Screenshot docs/tui-review/frames/start-filter.png
Sleep 250ms

Escape
Sleep 200ms
Type "?"
Wait+Screen /about lookit/
Screenshot docs/tui-review/frames/help.png
Sleep 250ms

Type "a"
Wait+Screen /modern TUI browser/
Screenshot docs/tui-review/frames/about.png
Sleep 250ms

Escape
Sleep 200ms
Type "q"
Sleep 200ms
