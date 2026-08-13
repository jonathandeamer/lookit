# Shared walkthrough for the responses review tapes. No Set/Output here —
# each responses-*.tape applies size and theme first, then Sources this file.
#
# Needs the loopback fingerd on 127.0.0.1:2479 and :2480 (make review-tui
# starts it; this Hide block starts it if you record a tape by hand).
# Sleep after every Screenshot: VHS captures the *next* frame.

Hide
Type "export XDG_CONFIG_HOME=$PWD/docs/tui-review/fixtures/xdg"
Enter
Type "export PS1='$ '"
Enter
Type "test -x ./lookit || go build -o lookit ."
Enter
Sleep 1s
Type "test -x docs/tui-review/fixtures/fingerd/fingerd || go build -o docs/tui-review/fixtures/fingerd/fingerd ./docs/tui-review/fixtures/fingerd"
Enter
Sleep 2s
# -ping verifies the fixture bodies, so a port held by something else fails
# the recording instead of being adopted as if it were the fixture.
Type "docs/tui-review/fixtures/fingerd/fingerd -ping >/dev/null 2>&1 || docs/tui-review/fixtures/fingerd/fingerd &"
Enter
Sleep 600ms
Type "docs/tui-review/fixtures/fingerd/fingerd -ping || exit 1"
Enter
Sleep 400ms
Type "clear"
Enter
Show

Type "./lookit @127.0.0.1:2479"
Enter
Wait+Screen /Users currently online/
Screenshot docs/tui-review/frames/list.png
Sleep 250ms

Enter
Wait+Screen /Alice Review/
Screenshot docs/tui-review/frames/reader.png
Sleep 250ms

Tab
Wait+Screen /link 1/
Screenshot docs/tui-review/frames/reader-link.png
Sleep 250ms

Type "L"
Wait+Screen /example.com/
Screenshot docs/tui-review/frames/links.png
Sleep 250ms

Escape
Sleep 200ms
Type "v"
Wait+Screen /esc back/
Screenshot docs/tui-review/frames/raw.png
Sleep 250ms

Escape
Sleep 200ms
Type "i"
Wait+Screen /esc cancel/
Screenshot docs/tui-review/frames/reader-input.png
Sleep 250ms

Escape
Sleep 200ms
Type "i"
Sleep 200ms
Ctrl+U
Type "@127.0.0.1:2480"
Enter
Wait+Screen /auto-detected/
Screenshot docs/tui-review/frames/generic.png
Sleep 250ms

Type "i"
Sleep 200ms
Ctrl+U
Type "trunc@127.0.0.1:2479"
Enter
Wait+Screen /partial/
Screenshot docs/tui-review/frames/truncated.png
Sleep 250ms

Type "i"
Sleep 200ms
Ctrl+U
Type "nobody@127.0.0.1:1"
Enter
# "connection refused" wraps on 60 columns; the dial verb does not.
Wait+Screen /connect:/
Screenshot docs/tui-review/frames/error.png
Sleep 250ms

Type "q"
Sleep 200ms
