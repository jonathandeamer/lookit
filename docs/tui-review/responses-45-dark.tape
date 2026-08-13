# 45×24, lookit dark. Same cell geometry as chrome-45-dark.tape.
# Responses at the narrow floor: the reader, the links panel, the help
# overlay's column count, and the dial error's wrap all get their hardest
# width here.
#
# Needs the loopback fingerd (make review-fingerd; the tour starts it).
# From the repo root:  vhs docs/tui-review/responses-45-dark.tape


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

Source docs/tui-review/responses-tour.tape
