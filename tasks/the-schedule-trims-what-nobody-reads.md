# the-schedule-trims-what-nobody-reads

The schedule pass, the one thing that runs every minute whether or not a session is up, should
clean up after the forge - and right now nothing does.

The operator read the tmp sweep as worth doing - "that sounds like a good thing to queue" -
and asked for the schedule's own log to be bounded by day, wanting to know what a week costs.
The measurement: the log was born 2026-09-23 14:35:30 and is 33.7 MB three days and six hours
later, so about 10.3 MB a day and 72 MB a week at this week's rate; a single pass is ~6 KB, so
the steady floor is 8.7 MB a day and 61 MB a week. A week of it is 60-70 MB.

What accumulates and nobody removes:

  tmp/swarmforge-compose-*/   a compose's scratch, removed when the run succeeds and left when
                              it fails, deliberately, "so a person can look at it"
  tmp/update-exercise-*/      a failed update exercise's work, left for the same reason
  .swarmforge/stall-watch.log the schedule's own output, now 33.7 MB and growing every minute

The rule the sweep has to follow is age - untouched since - rather than a name pattern: the
names are honest today, but a pattern deletes the wrong thing one day, and the honest question
about a failed run's scratch is whether anyone is still looking at it. The forge has a
precedent with the other axis and the same spirit: prune_build_debris.sh keeps the newest
twenty scenario runs and the newest mutant run per directory, "so a failure can still be
looked at". Nothing runs that one either.

The log is the exception to age: it is written every minute, so its mtime is always now and an
age rule would never touch it. It wants rotation by file instead - a date-named file per day,
with the pass deleting its own older than seven - which works because launchd spawns a fresh
process each minute and opens the path each time. Its lines carry no timestamps, so a cutoff
inside the file is not available to us.

The acceptance worth having: a pass with an old scratch directory in a forge's tmp, an old
date-named log beside today's, and something current in both leaves the current things alone,
removes the old scratch, and drops the log older than the bound - and reports what it did, so
the removal is visible in the next pass's output rather than silent.

The projects are the second half, and the operator asked whether this would reach them. It
should, because the pass is the only thing already walking every project of every forge it
serves, and because nothing runs the forge's own pruner today: prune_build_debris.sh would
free 55 MB of 126 directories in this forge and 43 MB of 710 in Saibill as I write this, and
the same script was written for a 27 GB measurement on 2026-09-23. Its rule is the other axis
and already decided - keep the newest twenty scenario runs and the newest mutant run per
directory, "so a failure can still be looked at" - so the pass does not need a policy for
project debris, only a cadence and a home for it.

Two guards come with reaching into projects. A project prune is not a thing to do every minute:
the pass should run it once a day, gated on the date, so the walk stays cheap. And the pruner
is the layer's script while the pass is the kit's, so the pass calls it only when it is there
and says so when it is not - a forge composed from another family's layer has no such script,
and the kit must not break on one.

Third, in the operator's own words: stay out of the Maven target directories. Never `target/`,
in a project's tree or a worktree's, and never the binaries in `build/acceptance/bin` either.
The two paths this may touch are `build/acceptance/run` and
`build/acceptance-mutation/<feature>/mutations`, and nothing else - a project's own build
output is the project's, and Saibill's Maven projects carry 36 MB of it in each of saibill-sgo's
six worktrees and 13 MB in each of saibill's, some of which is what a Maven deploy publishes.
The count-based rule already wanders nowhere, so this binds the age half, which is the half
that could; it is named in the note rather than left to the pruner's own restraint.
