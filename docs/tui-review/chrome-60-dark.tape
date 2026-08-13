# 60×20, lookit dark. Pixel size calibrated at FontSize 18 / Padding 0:
# 686×444 → 60 columns × 20 rows. Below startWideMinWidth (72), so the
# startpage uses the stacked narrow layout.
#
# From the repo root:  vhs docs/tui-review/chrome-60-dark.tape
# Or record every tape:  make review-tui

Require vhs

Output docs/tui-review/frames/_render.gif

Set Width 686
Set Height 444
Set FontSize 18
Set FontFamily "JetBrainsMono NF"
Set Padding 0
Set Margin 0
Set CursorBlink false
Set Framerate 12
Set TypingSpeed 1ms
Set Theme { "name": "lookit-dark", "background": "#171719", "foreground": "#f0edf5", "black": "#25222a", "brightBlack": "#766f7d", "red": "#ff6f87", "brightRed": "#ff6f87", "green": "#38e7ad", "brightGreen": "#38e7ad", "yellow": "#eed76d", "brightYellow": "#eed76d", "blue": "#9878ff", "brightBlue": "#9878ff", "magenta": "#ff5fa2", "brightMagenta": "#ff5fa2", "cyan": "#38e7ad", "brightCyan": "#38e7ad", "white": "#f0edf5", "brightWhite": "#fbfafc", "selection": "#342747", "cursor": "#9878ff" }

Source docs/tui-review/tour.tape
