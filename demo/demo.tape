# demo.tape - VHS script for the README hero gif (demo/demo.gif).
#
# Record from the repo root:  vhs demo/demo.tape
# Needs vhs, ttyd, and ffmpeg on PATH:  brew install vhs ffmpeg ttyd
#
# This fingers tilde.team and graph.no live, so the Sleep values around fetches
# assume a working connection. The TUI and network can't be tested headlessly,
# so treat the timings as a starting point and tune them after the first record.

Output demo/demo.gif

# --- Look ----------------------------------------------------------------
# Render at 2x the README's 600px display width for retina crispness.
Set Width 1200
Set Height 750
# 18 (down from 22) so tilde.team's welcome banner leaves room for more than a
# single user row in the list. Trade-off: smaller text when scaled to 600px.
Set FontSize 18
# "JetBrainsMono NF" is the installed family name for JetBrainsMono Nerd Font
# (brew install --cask font-jetbrains-mono-nerd-font). Renders the wordmark.
Set FontFamily "JetBrainsMono NF"
Set Padding 20

# Lighter file for a README hero, and a calm cursor (no blink shimmer).
Set Framerate 24
Set CursorBlink false

# macOS-style window card, framed in lookit's brand violet.
Set WindowBar Colorful
Set BorderRadius 12
Set Margin 40
Set MarginFill "#6d43d6"

# lookit's own dark palette: the terminal background is the app's #171719 base,
# so the card is seamless and the ANSI colours echo the in-app gradient.
Set Theme { "name": "lookit", "background": "#171719", "foreground": "#f0edf5", "black": "#25222a", "brightBlack": "#766f7d", "red": "#ff6f87", "brightRed": "#ff6f87", "green": "#38e7ad", "brightGreen": "#38e7ad", "yellow": "#eed76d", "brightYellow": "#eed76d", "blue": "#9878ff", "brightBlue": "#9878ff", "magenta": "#ff5fa2", "brightMagenta": "#ff5fa2", "cyan": "#38e7ad", "brightCyan": "#38e7ad", "white": "#f0edf5", "brightWhite": "#fbfafc", "selection": "#342747", "cursor": "#9878ff" }

Set TypingSpeed 80ms

# --- Install the released v0.1.0 off-camera ------------------------------
# Use the tagged release, not a local dirty build, so the About screen shows a
# clean 'v0.1.0' (the build-date row hides itself when the date is unknown).
# go install puts the binary on $PATH (~/go/bin), so the visible command below
# is a clean 'lookit' rather than './lookit'.
Hide
Type "export PATH=$HOME/go/bin:$PATH"
Enter
Type "export PS1='$ '"
Enter
Type "go install github.com/jonathandeamer/lookit@v0.1.0"
Enter
Sleep 5s
Type "clear"
Enter
Show

# --- Open a host; lookit lists its users ---------------------------------
Type "lookit @tilde.team"
Sleep 600ms
Enter
Sleep 3s

# Page forward through the list, then step back a couple. Quick, to show it moves.
Right@300ms 5
Sleep 700ms
Left@300ms 2
Sleep 700ms

# --- Filter to a user and drill in ---------------------------------------
Type "/"
Sleep 400ms
Type "jon"
Sleep 1200ms
# First Enter confirms the filter (jonathan is now selected); the list owns
# keys while filtering, so a second Enter is what drills into the user.
Enter
Sleep 800ms
Enter
Sleep 3s

# Walk back to the list without re-fetching.
Escape
Sleep 1500ms

# --- Context-aware help, then about --------------------------------------
Type "?"
Sleep 2500ms
Type "a"
Sleep 2500ms

# Back to the list.
Escape
Sleep 1200ms

# --- Finger a different host from the target input -----------------------
# 'i' focuses the input pre-filled with the current target, so clear it first.
Type "i"
Sleep 500ms
Ctrl+U
Sleep 300ms
Type "liverpool@graph.no"
Sleep 600ms
Enter
Sleep 3500ms

# Let the final frame breathe.
Sleep 1500ms
