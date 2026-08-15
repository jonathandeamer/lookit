#!/bin/sh
# Validate one Conventional Commits subject line:
#
#   <type>(<optional-scope>)<optional !>: <description>
#
# The single source of truth for lookit's commit vocabulary. Two callers share
# it so they can't drift: .githooks/commit-msg (local, on `git commit`) and
# .github/workflows/commit-style.yml (CI, on the PR title — which is what a
# squash merge writes onto main, where the local hook never runs).
#
# Usage: scripts/check-conventional-subject.sh "<subject>" [label]
# `label` names the thing being checked in the error message ("Subject" by
# default, "PR title" from CI). Exits 0 if valid, 1 with an explanation if not.

subject="$1"
label="${2:-Subject}"

# The standard Conventional Commits set.
types='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert'

# Scopes are optional, but when given must name a real part of lookit. Keeping
# this closed catches typos and drift (feat(ui) vs feat(tui)); to add a scope,
# add it here — that's the intended way to grow the list.
#
#   packages/subsystems  finger render tui startpage catalog theme cli main
#   docs areas           spec plan superpowers releasing security
#   tooling              deps release demo tui-review actions dependabot
scopes='finger|render|tui|startpage|catalog|theme|cli|main'
scopes="$scopes|spec|plan|superpowers|releasing|security"
scopes="$scopes|deps|release|demo|tui-review|actions|dependabot"

if printf '%s\n' "$subject" | grep -Eq "^($types)(\(($scopes)\))?!?: .+"; then
	exit 0
fi

# Split the diagnosis so the message points at the actual mistake: an unknown
# scope inside otherwise-valid syntax is a different fix from bad syntax.
if printf '%s\n' "$subject" | grep -Eq "^($types)\([a-zA-Z0-9._/-]+\)!?: .+"; then
	bad_scope=$(printf '%s\n' "$subject" | sed -E 's/^[a-z]+\(([^)]*)\).*/\1/')
	cat >&2 <<EOF
✗ Unknown commit scope: ($bad_scope)

  $label: $subject

  Scopes:  $(printf '%s' "$scopes" | tr '|' ' ')

  Scope is optional — drop it, or add the new one to
  scripts/check-conventional-subject.sh if it names a real part of lookit.
EOF
	exit 1
fi

cat >&2 <<EOF
✗ Does not follow Conventional Commits.

  $label: $subject

  Expected: <type>(<optional-scope>): <description>
  Types:    $(printf '%s' "$types" | tr '|' ' ')
  Scopes:   $(printf '%s' "$scopes" | tr '|' ' ')
  Examples: feat(tui): add raw-body view to the best-guess list
            fix(finger): pin server-supplied targets to :79
            ci: bump actions to v6

  Scope is optional.
EOF
exit 1
