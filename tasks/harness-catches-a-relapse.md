# harness-catches-a-relapse

Nothing fails today if the bridge stops asking the dashboard and goes back to
writing the forge's queue files itself.

The wake card fixed that behaviour by making the bridge post to the dashboard's
own chat endpoint, and the acceptance suite does assert the observable result —
the dashboard holds the request and the wake reaches the pane. But both of those
are also true of the old queue-writing path, which is exactly how the defect
passed a green suite the first time: the harness could not tell the two paths
apart. The fixture-git-isolation card just taught the suite the same lesson from
the other direction, about git rather than about who writes the queue.

Desired behaviour: a bridge that writes the queue itself, instead of asking the
dashboard to, turns the suite red. Keep the harness driving the production path
as it does now, and prove the guard fires — reintroduce the queue write, watch
the suite fail, then take it back out. If a scenario can only show the
observable result, it should at least name which writer it is asserting on.

This is the bridge's own harness, not a change to the forge's base tooling.
