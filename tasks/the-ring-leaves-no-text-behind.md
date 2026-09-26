# the-ring-leaves-no-text-behind

The doorbell's ring can end up sitting in the pane's composer, unsent, and the doorbell
counts it as delivered anyway.

The operator reported text they did not type appearing in both lieutenants' panes and
having to be deleted before they could type, and then that the third ring of Saibill's
forwarded clarification "was sitting in saibill's pane without being submitted". The
timeline is in the ledger: that clarification was rung at 16:39 and again at 16:49:59, and
the pane holds one copy of the ring text - the first ring's, the one the Saibill lieutenant
read. The later rings are the text that sat in the composer: the second was deleted by the
operator, the third was only submitted by hand at 17:40, fifty minutes after it was typed.

The ring types the whole request - Saibill's is about 3.5 KB of multi-line prose - and then
presses Enter with no pause at all (the dashboard's own wake waits between the two, which
is the only reason this shows up here first). The TUI is still taking the paste when the
Enter arrives, so the Enter is lost and the text stays in the composer.

The worse half is what the doorbell believes afterwards. Its proof of delivery reads the
pane's screen, and the composer is part of the screen, so a ring whose text never left the
composer still counts as delivered - and because the first ring's submitted copy is also in
the scrollback, every later ring is "already delivered from the scrollback and left alone".
A request can therefore be counted as delivered when no session has seen it, which is the
failure the doorbell exists to prevent.

What the ring should do: put the text in as one bracketed paste, so a multi-line body cannot
submit itself on its newlines, then press Enter once; and check the result - an empty
composer means the ring landed, text still there means it did not, and that ring is either
retried or reported as not landed rather than counted. Text found only in the composer is
not evidence that anything was delivered.

The acceptance worth having: a ring whose text is a long multi-line clarification puts one
submitted turn in the pane and leaves the composer empty; a ring whose Enter is lost leaves
the request owed and says so, rather than recording a delivery.
