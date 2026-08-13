#!/bin/sh
# Sanity-check one recorded tape directory of stills.
#
# `make review-tui` already checks that a tape produced as many PNGs as its
# tour asks for. That count says nothing about what is *in* them: VHS writes
# a still from a frame queue that lags the live terminal, so a Screenshot
# taken too soon after a Wait banks an older screen — in practice a blank
# frame (the TUI had not painted) or a duplicate of the previous still.
#
# Two cheap checks catch both shapes:
#   blank     a still far smaller than its neighbours is a near-empty screen;
#             PNG compresses flat colour to almost nothing (a blank 80x24
#             frame measured ~3 KB against ~70 KB for a painted one).
#   duplicate two identical stills mean one scene captured the other's screen.
#
# Neither catches a still that is plausible but wrong (a real screen, one step
# behind). Only the Wait/Sleep/Screenshot order in the tapes prevents that,
# and only a person looking at the frames confirms it.
#
# Usage: verify-frames.sh <frames dir>

set -eu

dir=${1:?usage: verify-frames.sh <frames dir>}

# A still under this fraction of the directory's median size (in percent) is
# treated as blank. Real stills measured 60-100% of their median; blank ones
# came in around 4%, so the gap is wide and the threshold is not delicate.
min_percent=25

count=$(find "$dir" -name '*.png' -type f | wc -l | tr -d ' ')
if [ "$count" -eq 0 ]; then
	echo "$dir: no stills to verify" >&2
	exit 1
fi

sizes=$(find "$dir" -name '*.png' -type f -exec wc -c {} + |
	grep -v ' total$' | awk '{print $1}' | sort -n)
median=$(echo "$sizes" | awk -v n="$count" 'NR == int((n + 1) / 2) {print $1}')

status=0

for png in "$dir"/*.png; do
	size=$(wc -c <"$png" | tr -d ' ')
	percent=$((size * 100 / median))
	if [ "$percent" -lt "$min_percent" ]; then
		echo "$png: $size bytes is $percent% of the median $median — blank frame?" >&2
		echo "  (Screenshot captured before the screen painted; see the Sleep note in the tour tape)" >&2
		status=1
	fi
done

dupes=$(for png in "$dir"/*.png; do
	echo "$(cksum <"$png" | awk '{print $1, $2}') $png"
done | sort | awk '{ key = $1 " " $2; if (key == prev) print prevfile, $3; prev = key; prevfile = $3 }')

if [ -n "$dupes" ]; then
	echo "$dir: identical stills — one scene captured another's screen:" >&2
	echo "$dupes" >&2
	status=1
fi

exit $status
