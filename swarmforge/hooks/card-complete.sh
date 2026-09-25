#!/usr/bin/env bash
#
# Card complete: a finished card reaches GitHub without anyone remembering to push.
#
# The tooling runs this hook once per card, in the worktree that holds master, and
# only once that card's board row says done. Everything it knows arrives in the
# environment - SWARMFORGE_EVENT, SWARMFORGE_PROJECT, SWARMFORGE_TASK,
# SWARMFORGE_COMMIT, SWARMFORGE_FROM, SWARMFORGE_ROLE, SWARMFORGE_HOOK - and the
# hook takes no arguments, so it never expects a flag.
#
# What the event means is this project's business, and here it is one push: the
# composition builds a forge's bridge from what GitHub has, so the branch the
# card landed on goes to origin and the code a forge downloads is the code this
# bridge runs. A push that fails says so and exits non-zero, which is how the
# tooling comes to print HOOK_FAILED, so a card is never left looking pushed when
# it is not.
#
# It never forces. A remote that has moved on is a push that failed, reported as
# one, rather than work overwritten: what the other checkout put there is not
# this hook's to discard. A repository with no origin is not a failure - there is
# nowhere to send the work rather than work that was lost - so it says so and
# exits zero. A branch origin already holds is nothing to push rather than a push
# worth announcing.
#
# Deploying a running bridge is not this hook's business: the binary a bridge is
# serving from still has to be replaced when a card changed it, and this push says
# nothing about that either way. The gates are not repeated either - the pack ran
# them before the work reached master, and finishing a card must not pay for the
# suite twice.

set -u

project=${SWARMFORGE_PROJECT:-$(pwd)}
card=${SWARMFORGE_TASK:-the card}

cd "$project" || { echo "card-complete: cannot reach $project" >&2; exit 1; }

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "card-complete: $project is not a git repository, so $card has nowhere to go"
  exit 0
fi

branch=$(git rev-parse --abbrev-ref HEAD)

if ! git remote get-url origin >/dev/null 2>&1; then
  echo "card-complete: $card landed on $branch, and there is no remote to push to"
  exit 0
fi

here=$(git rev-parse HEAD)
there=$(git ls-remote origin "refs/heads/$branch" 2>/dev/null | cut -f1)

if [ -n "$there" ] && [ "$here" = "$there" ]; then
  echo "card-complete: origin already holds $branch for the card $card, so there was nothing new to push"
  exit 0
fi

if push=$(git push origin "$branch" 2>&1); then
  echo "card-complete: pushed $branch to origin for the card $card"
  exit 0
fi

echo "card-complete: the push failed, so $card is merged here but not on origin" >&2
echo "$push" >&2
exit 1
