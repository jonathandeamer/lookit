# 80×24, lookit light. Same cell geometry as chrome-80-dark.tape. The
# terminal background is the light BaseBg so Bubble Tea's BackgroundColorMsg
# should flip the adaptive palette.
#
# From the repo root:  vhs docs/tui-review/chrome-80-light.tape
# Or record every tape:  make review-tui

Require vhs

Output out/tui-review/_render.txt

Set Width 914
Set Height 580
Set FontSize 18
Set FontFamily "JetBrainsMono NF"
Set Padding 0
Set Margin 0
Set CursorBlink false
Set Framerate 12
Set TypingSpeed 1ms
Set Theme { "name": "lookit-light", "background": "#fbfafc", "foreground": "#25222a", "black": "#25222a", "brightBlack": "#766f7d", "red": "#c82f4d", "brightRed": "#c82f4d", "green": "#007f62", "brightGreen": "#007f62", "yellow": "#765f00", "brightYellow": "#765f00", "blue": "#6d43d6", "brightBlue": "#6d43d6", "magenta": "#c92870", "brightMagenta": "#c92870", "cyan": "#007f62", "brightCyan": "#007f62", "white": "#fbfafc", "brightWhite": "#ffffff", "selection": "#f3e9f4", "cursor": "#6d43d6" }

Source docs/tui-review/tour.tape
