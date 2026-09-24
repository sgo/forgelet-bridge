# install-kit-onto-a-stopped-forge

The kit installer refuses a forge that is not running, or one with a stopped project,
and there is nothing wrong with either.

Today's rule, read from the installer: it runs the idler check over every project the
forge serves and fails the install if any project reports "the forge is not running",
reads no role at all, or has a role whose pane is gone. Only the middle of those three is
a fault. A forge between sessions is normal, a project that is closed is normal, and a
forge being set up before it is started is the order most people would choose — install
the tools, then start the forge. As it stands, installing onto a stopped forge fails, and
Saibill would fail today: `saibill-spi` and `saibill-neutral` have no sessions, and
`saibill-api` has two panes where the others have six.

Where the rule came from is worth recording, because the intent was narrower: a
self-check exists so that a tool which reads nothing cannot pass as an install. A pane
was included because the pane read is the signal most worth proving — it is the one that
broke when Claude Code started reporting its version instead of its name — but including
it turned "the tool can read nothing" into "every project must have a live pane".

Desired behaviour: installing onto a forge that is not running, or whose projects are
stopped, succeeds and says what it could not exercise — naming the projects it read and
the pane path it could not prove, so a later run can prove it. Failing stays for the tool
that cannot find the forge's structure at all: no roles file, no board, no inbox to read,
which is a wrong path or a wrong forge rather than a quiet one. The busy and quiet reads
that already pass keep passing.
