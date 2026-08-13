# 45×24, lookit dark. Pixel size calibrated at FontSize 18 / Padding 0:
# 520×580 → 45 columns × 24 rows.
#
# The narrow floor. 60 columns is already below startWideMinWidth (72), but
# 45 is what a tmux split or a side-by-side iTerm pane on a laptop actually
# gives you, and it is where a two-column layout has nowhere left to go.
# Same 24 rows as the 80-column tapes, so width is the only variable.
#
# From the repo root:  vhs docs/tui-review/chrome-45-dark.tape
# Or record every tape:  make review-tui


Require vhs

Output out/tui-review/_render.txt

Set Width 520
Set Height 580
Set FontSize 18
Set FontFamily "JetBrainsMono NF"
Set Padding 0
Set Margin 0
Set CursorBlink false
Set Framerate 12
Set TypingSpeed 1ms
Set Theme { "name": "lookit-dark", "background": "#171719", "foreground": "#f0edf5", "black": "#25222a", "brightBlack": "#766f7d", "red": "#ff6f87", "brightRed": "#ff6f87", "green": "#38e7ad", "brightGreen": "#38e7ad", "yellow": "#eed76d", "brightYellow": "#eed76d", "blue": "#9878ff", "brightBlue": "#9878ff", "magenta": "#ff5fa2", "brightMagenta": "#ff5fa2", "cyan": "#38e7ad", "brightCyan": "#38e7ad", "white": "#f0edf5", "brightWhite": "#fbfafc", "selection": "#342747", "cursor": "#9878ff" }

Source docs/tui-review/tour.tape
