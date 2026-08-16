#!/bin/sh
# Record one review tape into out/tui-review/<tape name>/, and say plainly
# whether it worked. `make review-tui` calls this once per tape and keeps going
# when it fails, so this script owns everything about a single recording and
# the Makefile owns only the tape list.
#
# Run from the repo root (the tapes' Output paths are repo-relative):
#   sh docs/tui-review/record-tape.sh docs/tui-review/chrome-80-dark.tape 12
#
# WHY IT RETRIES. vhs drops its ttyd socket often enough that a 12-tape run
# rarely finished: "use of closed network connection" mid-recording, a
# different tape each time, and every one of them recording perfectly on its
# own. It is a flake in the harness, not in the scene, and this is a local
# review tool rather than a CI gate — so one retry, after a pause long enough
# for the previous ttyd to finish going away, is the right size of fix. A tape
# that fails twice is reported and the run moves on to the next one.
#
# A failed tape leaves NO directory behind. Stale frames from an earlier run
# would otherwise be tiled into this run's contact sheet and read as current,
# which is worse than a missing sheet: the whole premise of the kit is that
# every run starts from the same state.

set -u

tape=${1:?usage: record-tape.sh <tape> <want stills>}
want=${2:?usage: record-tape.sh <tape> <want stills>}

name=$(basename "$tape" .tape)
stage=out/tui-review
dest=$stage/$name

# Clear the staging root so a tape can never file the previous tape's frames
# under its own name.
clear_stage() {
	rm -f "$stage"/*.png "$stage"/_render.txt
}

record() {
	clear_stage
	vhs "$tape" || return 1
	got=$(find "$stage" -maxdepth 1 -name '*.png' -type f | wc -l | tr -d ' ')
	if [ "$got" != "$want" ]; then
		echo "$name: recorded $got stills, its tour asks for $want" >&2
		echo "  (a Screenshot with no following Sleep writes nothing)" >&2
		return 1
	fi
	return 0
}

if ! record; then
	echo "$name: recording failed, retrying once" >&2
	sleep 3
	if ! record; then
		echo "$name: recording failed twice, skipping" >&2
		clear_stage
		rm -rf "$dest"
		exit 1
	fi
fi

mkdir -p "$dest"
rm -f "$dest"/*.png
mv "$stage"/*.png "$dest"/
rm -f "$stage"/_render.txt

sh docs/tui-review/verify-frames.sh "$dest" || exit 1
