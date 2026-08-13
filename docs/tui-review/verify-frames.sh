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
#   blank     a still far smaller than the fullest one in its tape is a
#             near-empty screen; PNG compresses flat colour to almost nothing
#             (a blank 80x24 frame measured ~3 KB against ~70 KB painted).
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
# A still under this fraction of the tape's LARGEST still (in percent) is
# treated as blank.
#
# The comparison is against the maximum, not the median, because several
# scenes are legitimately sparse: a filter matching nothing (Bubbles renders
# an empty Filtering list as the empty string), the last page of the list
# holding one row, and `catalog off` showing one bookmark — the last of which
# is most of a 100x50 frame's height. Against a median those read as blank;
# against the fullest frame in the same tape they are plainly painted.
#
# Measured, all stills normalised to the fullest frame of their own tape:
#   truly blank            1-2%
#   legitimately sparse   13-21%
#   ordinary painted      50-100%
# 5% sits in the wide gap below everything real, so the threshold is not
# delicate. Raising it to "catch more" would start failing honest frames.
min_percent=5

count=$(find "$dir" -name '*.png' -type f | wc -l | tr -d ' ')
if [ "$count" -eq 0 ]; then
	echo "$dir: no stills to verify" >&2
	exit 1
fi

largest=$(find "$dir" -name '*.png' -type f -exec wc -c {} + |
	grep -v ' total$' | awk '{print $1}' | sort -n | tail -1)

status=0

for png in "$dir"/*.png; do
	size=$(wc -c <"$png" | tr -d ' ')
	percent=$((size * 100 / largest))
	if [ "$percent" -lt "$min_percent" ]; then
		echo "$png: $size bytes is $percent% of the largest still $largest — blank frame?" >&2
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
