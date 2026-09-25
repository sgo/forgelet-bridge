# push-the-bridge-when-a-card-lands

The composition builds the bridge from what GitHub has, so a bridge card that lands
without a push leaves the next forge built from yesterday's bridge, and nothing in
that composed forge says so.

What the bridge needs is the step the app already has: when a card's work lands on
its master, push the repository, so the code a forge downloads is the code the
bridge runs. A push that fails should say so rather than pass quietly, and a card
whose work is already pushed should not mind being pushed again.

Deploying the running bridge is a different question and not this card: the binary
that bridge is serving from still needs replacing when a card changed it.
