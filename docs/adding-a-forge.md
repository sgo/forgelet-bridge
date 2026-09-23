# Adding a forge

One bridge process serves every forge in its configuration: each forge gets its
own Matrix space with the rooms the operator handles it in, and its own name in
the space so a phone can tell the forges apart. Bringing another forge in is one
entry in the configuration and a restart — this runbook is the whole path, so
somebody who has never done it can.

The operator's entry point is the forge's own adapter,
`<forge root>/swarmforge/scripts/matrix-bridge.sh`, which serves the forge root
it lives in. The commands below are this repository's adapter, called the same
way for the forge root it serves; the adapter never assumes it is running from
its own repository's directory. Set the two names the rest of this runbook uses:

```sh
root="<forge root this bridge serves>"          # the directory holding .swarmforge/matrix-bridge.json
adapter="<this repository>/scripts/matrix-bridge.sh"
```

## 1. What the target forge must have

- **A running dashboard that has announced itself.** The bridge finds a forge's
  dashboard through `<forge root>/.swarmforge/dashboard-url`, and reads it fresh
  every time, so the entry there has to point at the dashboard that is running
  now. A dashboard that was stopped leaves a stale address behind, which is what
  "unreached" means below.
- **The endpoints the bridge calls.** Check them against the dashboard's
  address before you add the forge:

  | Endpoint | What the bridge uses it for |
  | --- | --- |
  | `GET /api/state` | the approvals the forge's open projects are waiting for |
  | `POST /api/chat` | a phone message, which the dashboard queues and wakes the lieutenant with |
  | `POST /api/approvals/<id>/approve` | an approval the operator approved from the phone |
  | `POST /api/tasks/retry` | an approval the operator sent back, with their feedback |
  | `POST /api/clarifications/<id>/answer` | a clarification the operator answered, which wakes the blocked role |

  ```sh
  url="$(cat "$root/.swarmforge/dashboard-url")"
  curl -sS "$url/api/state" >/dev/null && echo "state is there"
  ```
- **Older tooling.** A forge whose dashboard does not carry one of these
  endpoints is not ready to be added. Update that forge's own tooling first (its
  dashboard is the forge's, not this bridge's), or leave the forge out of the
  configuration. A forge that is missing an endpoint shows up as that one forge
  refusing actions and being named as unhappy in the status — it does not quiet
  the forges that are working, but it also cannot be carried until its tooling
  answers. The five endpoints above are the whole contract; nothing else about
  the forge has to change.
- **The operator.** One bridge serves one operator account, already configured.
  Nothing per-forge is needed for it.

## 2. Add the entry

The adapter does the edit, the copy and the restart in one step:

```sh
MATRIX_BRIDGE_FORGE_ROOT="$root" "$adapter" add-forge "<root of the forge to add>" "<name the operator knows it by>"
```

For the Saibill forge beside this one:

```sh
MATRIX_BRIDGE_FORGE_ROOT=/Users/sgo/forgelet-forge "$adapter" add-forge /Users/sgo/sgo Saibill
```

What it does, in this order:

1. reads `.swarmforge/matrix-bridge.json` and adds `{"root": "<root>", "name":
   "<name>"}` to `forges`;
2. validates the result before anything is written — a root that is blank, or a
   root that is already configured, is refused and the configuration is left
   exactly as it was;
3. keeps a copy of the configuration it replaced (this copy is the rollback);
4. writes the new configuration, restarts the bridge, and echoes the forge it
   added, together with the copy it kept.

The command is invoked for the forge root it serves; it never assumes it is
running from this repository's own directory. If it refuses, read the reason it
prints; nothing was changed and the bridge is still carrying what it carried.
Run the same command again once the entry is fixed, or edit the configuration by
hand and restart.

## 3. Check it worked

```sh
MATRIX_BRIDGE_FORGE_ROOT="$root" "$adapter" status
```

The status carries the bridge's startup report: every configured forge it
reached, and every one it did not. The forge you just added has to be named as
reached. A forge it did not reach is named as unreached and retried on its own —
the other forges keep working while you fix it.

Then look at the phone: the new forge has its own space, named with the name you
gave it, holding its chat room, approvals room, activity room and clarifications
room, with the operator invited.

## 4. Smoke test both directions

1. **A message out and a message in.** From the phone, send a message into the
   new forge's chat room. It has to become a chat request that *that forge's*
   dashboard takes and types into *its* lieutenant's pane — not the other
   forge's. Answer it and the answer comes back as a reply in the thread.
2. **An approval.** A handoff waiting for approval in one of the new forge's
   open projects appears in its approvals room. Tapping ✅ approves it exactly as
   the desktop does; replying in its thread sends it back with that feedback.
3. **A clarification.** A clarification one of the new forge's agents raised
   appears in its clarifications room. Replying in its thread carries the answer
   back through the dashboard, and the blocked agent wakes with it.

## 5. Roll back

```sh
MATRIX_BRIDGE_FORGE_ROOT="$root" "$adapter" stop
cp "<the copy add-forge named>" "$root/.swarmforge/matrix-bridge.json"
MATRIX_BRIDGE_FORGE_ROOT="$root" "$adapter" start
```

The bridge stops carrying that forge. The space and rooms it provisioned stay in
Matrix — the bridge only adds — so remove them in Element if you want them gone.

## Where this is specified

`features/adding-a-forge.feature` for the adapter's edit and its refusal,
`features/forge-startup-report.feature` for the report the status prints, and
the per-room features (`chat-channel-relay`, `phone-approvals`,
`phone-clarifications`, `card-activity-feed`) for what the smoke test shows.
