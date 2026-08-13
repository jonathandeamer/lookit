# Shared walkthrough for the responses review tapes. No Set/Output here —
# each responses-*.tape applies size and theme first, then Sources this file.
#
# Needs the loopback fingerd on 127.0.0.1:2479 (make review-tui starts it;
# this Hide block starts it if you record a tape by hand).
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
Type "pgrep -f 'docs/tui-review/fixtures/fingerd/fingerd' >/dev/null || docs/tui-review/fixtures/fingerd/fingerd &"
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
