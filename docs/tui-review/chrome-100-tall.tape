# 100×50, lookit dark. Pixel size calibrated at FontSize 18 / Padding 0:
# 1132×1180 → 100 columns × 50 rows.
#
# The whole-page geometry: 50 rows fits BOOKMARKS, COMMUNITIES and every
# SERVICES group in one frame. Density, vertical rhythm and hierarchy are
# properties of the composition, and the 24- and 30-row tapes show only its
# top dozen rows — this is the tape that can answer "is the startpage
# crowded". It is also an honest size: a maximised terminal clears 50 rows.
#
# From the repo root:  vhs docs/tui-review/chrome-100-tall.tape
# Or record every tape:  make review-tui


Require vhs

Output out/tui-review/_render.txt

Set Width 1132
Set Height 1180
Set FontSize 18
Set FontFamily "JetBrainsMono NF"
Set Padding 0
Set Margin 0
Set CursorBlink false
Set Framerate 12
Set TypingSpeed 1ms
Set Theme { "name": "lookit-dark", "background": "#171719", "foreground": "#f0edf5", "black": "#25222a", "brightBlack": "#766f7d", "red": "#ff6f87", "brightRed": "#ff6f87", "green": "#38e7ad", "brightGreen": "#38e7ad", "yellow": "#eed76d", "brightYellow": "#eed76d", "blue": "#9878ff", "brightBlue": "#9878ff", "magenta": "#ff5fa2", "brightMagenta": "#ff5fa2", "cyan": "#38e7ad", "brightCyan": "#38e7ad", "white": "#f0edf5", "brightWhite": "#fbfafc", "selection": "#342747", "cursor": "#9878ff" }

Source docs/tui-review/tour.tape
