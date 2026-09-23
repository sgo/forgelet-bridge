# installing-brings-the-rules

The rules the bridge's rooms depend on live in files the bridge knows nothing
about, so installing the bridge for a forge leaves that forge's roles unaware of
them.

Two examples that already matter. A role that stops while holding a card is
supposed to raise a clarification, and that is the mechanism that lets the
operator move it back into work — a role that goes quiet cannot be helped. And
the approvals and clarifications rooms only work because the roles ask and answer
through them: the operator's swipe is an answer because the role asked a question
in the first place. Today the text for those rules is updated by hand, one copy at
a time, across this forge's constitution, each project's own tracked copy, and the
family pack that new projects are scaffolded from — which is how copies drift.

The operator's ask: installing the bridge for a forge should bring the rules with
it, as a tool rather than a checklist.

Desired behaviour: an idempotent installer step that adds or refreshes the prompt
rules the bridge relies on — one marked block per subject, so running it again is
safe and drift shows up as a diff rather than a surprise, and so a forge that has
deliberately changed its own constitution keeps its own wording. This forge
switched its language and tooling to Go and left parts of the base text alone;
the installer must respect that and only own what it wrote. It should report what
it changed, what was already current, and what it deliberately left alone, and it
must be safe to run on a forge whose projects are mid-card.

Scope boundary worth specifying: the installer owns the forge's own constitution
and the pack source that new projects come from. It does not reach into a live
project's tracked tree — that tree belongs to the pack's lanes, and a project
picks a new rule up the way it picks up any other change, through its specifier.

The first rule it installs is the one the operator just asked for: stopping
without finishing — a role that stops while holding a card raises a clarification
naming the card, what stops it, what it tried and what it needs, and a role with
nothing assigned reports NO_TASK instead of going quiet.
