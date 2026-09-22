# phone-message-wake

When I send a chat message from my phone it appears in the desktop dashboard,
but it never reaches you: nothing wakes you, so unless I come here and tell you
about it, the message just sits there. Sending from the phone has to reach you
exactly the way a message typed into the dashboard does — the wake included —
because otherwise the phone is a place messages go to die.

The acceptance suite should not have been able to pass while this was broken,
and it did: the bridge writes straight into the request queue instead of going
through the forge the way a client does, and the suite's stand-in for the
dashboard agreed with that — it invents the wake the real dashboard never gives
when a file simply appears. A test that can disagree with the real dashboard
about what delivering a message means is worse than no test, because it bought
confidence for a broken slice.

So: fix the delivery so a phone message reaches you with its wake, and fix the
test's relationship to the real dashboard so this class of divergence cannot
pass again — including the parts of the bridge that are correct today only
because the stand-in happens to behave the same way.
