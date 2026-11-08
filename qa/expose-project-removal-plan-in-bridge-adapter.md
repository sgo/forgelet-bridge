# Expose project-removal planning through the installed host

This procedure exercises the real project-removal path, not a fixture adapter:

1. Build and install the project adapter in the target Family.
2. Run adapter conformance and the read-only `remove-project-plan` operation.
3. Capture the managed project's dashboard endpoint, bridge room and a
   server-side room event, and place a unique marker inside the project.
4. Submit the same authenticated `DELETE` request the app uses to the
   configured Forgelet Host and wait for its terminal operation.
5. Verify the operation reaches `completed`, the runtime stops, the directory
   moves to macOS Trash, the authoritative Family refresh drops the project,
   the bridge state drops only that project, and the captured Matrix event is
   still readable while the operator and bridge have left the room.
6. Confirm a sibling project remains present and openable.

The script requires these environment inputs. Values are read at run time and
are not written into the procedure, operation records or captured output:

```text
FORGELET_E2E_FAMILY_ROOT
FORGELET_E2E_PROJECT
FORGELET_E2E_HOST_URL
FORGELET_E2E_HOST_TOKEN_FILE
FORGELET_E2E_SIBLING_PROJECT
```

The operation is deliberately destructive for the named disposable project.
It never permanently deletes the directory or Matrix room.
