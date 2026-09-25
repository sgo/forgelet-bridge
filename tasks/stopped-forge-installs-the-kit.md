# stopped-forge-installs-the-kit

# stopped-forge-installs-the-kit

A forge that has been composed but never started cannot install the kit: the
doorbell's self-check wants the doorbell to say what it read, a forge with no pane
has nothing to read, so the check fails, the install stops half done — the tools
installed, the rules not — and the composition that ran it ends non-zero.

The idler half of this was fixed on 2026-09-24 by `install-kit-onto-a-stopped-forge`:
a forge between sessions, or one with a closed project, no longer fails the install.
What is left is the pane. The self-check's marker is the doorbell saying "read the
pane", which a forge with no pane at all cannot say, and this project's own scenario
already promises the outcome — "the installer's output names the projects it read
and the pane it could not prove", and "the installer succeeded". A tool that really
reads nothing must still fail; a forge that has not been started has simply not been
started.

The forge side is waiting on it: composing a Forgelet forge runs install-kit and
install-rules, and today that composition ends non-zero here, with the rules never
installed.
