# demo.tape - VHS script for the README hero gif (demo/demo.gif).
# Visual review of chrome belongs in docs/tui-review/, not here.
#
# Record from the repo root:  vhs demo/demo.tape
# Needs vhs, ttyd, ffmpeg, Go, and JetBrains Mono Nerd Font.
#
# This uses live finger servers. Record only when tilde.team, the Backup Box,
# and Crossed Fingers are responding, then tune the Tab count and reading
# pauses against the responses actually shown.

Output demo/demo.gif
Require go

# --- Look ----------------------------------------------------------------
# Render at 2x the README's 600px display width for retina crispness.
Set Width 1200
Set Height 750
Set FontSize 18
Set FontFamily "JetBrainsMono NF"
Set Padding 20

# Lighter file for a README hero, and a calm cursor (no blink shimmer).
Set Framerate 24
Set CursorBlink false
Set WaitTimeout 12s

# macOS-style window card, framed in lookit's brand violet.
Set WindowBar Colorful
Set BorderRadius 12
Set Margin 40
Set MarginFill "#6d43d6"

# lookit's own dark palette: the terminal background is the app's #171719 base,
# so the card is seamless and the ANSI colours echo the in-app gradient.
Set Theme { "name": "lookit", "background": "#171719", "foreground": "#f0edf5", "black": "#25222a", "brightBlack": "#766f7d", "red": "#ff6f87", "brightRed": "#ff6f87", "green": "#38e7ad", "brightGreen": "#38e7ad", "yellow": "#eed76d", "brightYellow": "#eed76d", "blue": "#9878ff", "brightBlue": "#9878ff", "magenta": "#ff5fa2", "brightMagenta": "#ff5fa2", "cyan": "#38e7ad", "brightCyan": "#38e7ad", "white": "#f0edf5", "brightWhite": "#fbfafc", "selection": "#342747", "cursor": "#9878ff" }

Set TypingSpeed 80ms

# --- Prepare the v0.2 build and a disposable profile off-camera -----------
# An existing empty bookmarks file is deliberate: it leaves the catalog
# visible without adding lookit's first-run starter bookmarks. Nothing here
# reads or writes the recorder's real configuration.
Hide
Type "mkdir -p out/demo/bin out/demo/config/lookit"
Enter
Type ": > out/demo/config/lookit/bookmarks"
Enter
Type "go build -ldflags '-X main.version=0.2.0' -o out/demo/bin/lookit ."
Enter
Type "export PATH=$PWD/out/demo/bin:$PATH"
Enter
Type "export XDG_CONFIG_HOME=$PWD/out/demo/config"
Enter
Type "export PS1='$ '"
Enter
Type "clear"
Enter
Show

# --- Start with somewhere to explore -------------------------------------
Type "lookit"
Sleep 500ms
Enter
Wait+Screen /@plan.cat/
Sleep 1200ms

# Down leaves the target input and selects the first catalog row: @plan.cat.
Down
Sleep 700ms
Type "b"
Wait+Screen /bookmarked @plan.cat/
Sleep 1300ms

# --- Browse tilde.team and find Jonathan ---------------------------------
Type "i"
Sleep 400ms
Ctrl+U
Type "@tilde.team"
Sleep 500ms
Enter
Wait+Screen /[0-9]+ users/
Sleep 900ms

Type "/"
Type "jon"
Sleep 900ms
Enter
Sleep 500ms
Enter
Wait+Screen /finger:\/\/thebackupbox.net\/fingp/
Sleep 1700ms

# Walk the links in the response until the Backup Box finger link is focused.
# Confirm this count against Jonathan's live .plan before recording.
Tab@450ms 3
Sleep 800ms
Enter
Wait+Screen /fingp@thebackupbox.net/
Sleep 1600ms

# --- Let the interface explain itself ------------------------------------
Type "?"
Sleep 2200ms
Type "a"
Sleep 2200ms
Escape
Sleep 900ms
Type "h"
Wait+Screen /@plan.cat/
Sleep 1000ms

# The Crossed Fingers example is the final catalog row. Bubbles binds G to
# "go to end"; VHS 0.11 tokenizes an End command but does not implement it.
Type "G"
Wait+Screen /search\?gemini@crossed-fingers.andros.dev/
Sleep 900ms
Enter
Wait+Screen /Search results for "gemini":/
Sleep 1700ms

# Reuse the current target, changing only the remote search term.
Type "i"
Sleep 500ms
Ctrl+A
Right@80ms 7
Delete@100ms 6
Type "smolnet"
Sleep 700ms
Enter
Wait+Screen /Search results for "smolnet":/

# Finish on the results rather than an overlay or loading state.
Sleep 2200ms
