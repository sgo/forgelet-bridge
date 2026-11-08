#!/usr/bin/env bash
# Exercise project removal through the installed host and macOS Trash.
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
forge_root="${MATRIX_BRIDGE_FORGE_ROOT:-$(cd "$project_root/../.." && pwd)}"

required=(
  FORGELET_E2E_FAMILY_ROOT
  FORGELET_E2E_PROJECT
  FORGELET_E2E_HOST_URL
  FORGELET_E2E_HOST_TOKEN_FILE
  FORGELET_E2E_SIBLING_PROJECT
)
for name in "${required[@]}"; do
  [[ -n "${!name:-}" ]] || {
    echo "project-removal-e2e: missing $name" >&2
    exit 1
  }
done

family="${FORGELET_E2E_FAMILY_ROOT}"
project="${FORGELET_E2E_PROJECT}"
host_url="${FORGELET_E2E_HOST_URL%/}"
token_file="${FORGELET_E2E_HOST_TOKEN_FILE}"
sibling="${FORGELET_E2E_SIBLING_PROJECT}"
project_dir="$family/projects/$project"
marker_name=".swarmforge/project-removal-e2e-marker"
marker_value="$(basename "$family")/$project"

[[ "$(uname -s)" == "Darwin" ]]
[[ -x /usr/bin/trash ]]
[[ -d "$family/.swarmforge" ]]
[[ -f "$project_dir/.swarmforge/single-agent" ]]
[[ -f "$family/projects/$sibling/.swarmforge/single-agent" ]]
[[ -r "$token_file" ]]

work="$(mktemp -d "${TMPDIR:-/tmp}/forgelet-bridge-project-removal.XXXXXX")"
trap 'rm -rf "$work"; rm -f "$project_dir/$marker_name"' EXIT
meta="$work/meta.json"

host_config="$forge_root/.swarmforge/forgelet-host.json"
python3 - "$host_config" "$work/bridge-config" "$work/adapter-path" <<'PY'
import json
import pathlib
import sys

host = json.loads(pathlib.Path(sys.argv[1]).read_text())
pathlib.Path(sys.argv[2]).write_text(str(host["bridge_config"]))
pathlib.Path(sys.argv[3]).write_text(str(host["bridge_adapter"]))
PY
bridge_config="$(cat "$work/bridge-config")"
installed_adapter="$(cat "$work/adapter-path")"
[[ -r "$bridge_config" ]]

"$project_root/scripts/build.sh" >/dev/null
install -m 0755 "$project_root/scripts/matrix-bridge.sh" \
  "$installed_adapter"
"$project_root/scripts/check-adapter-conformance.sh" "$installed_adapter"

plan="$(
  MATRIX_BRIDGE_FORGE_ROOT="$forge_root" \
  MATRIX_BRIDGE_CONFIG="$bridge_config" \
  MATRIX_BRIDGE_BINARY="$project_root/build/acceptance/bin/forgelet-bridge" \
  "$installed_adapter" remove-project-plan "$family" "$project"
)"
[[ "$plan" == "ready" ]]
echo "project-removal-e2e: installed adapter planned removal"

mkdir -p "$project_dir/.swarmforge"
printf '%s\n' "$marker_value" > "$project_dir/$marker_name"

MATRIX_BRIDGE_CONFIG="$bridge_config" \
FORGELET_E2E_FAMILY_ROOT="$family" \
FORGELET_E2E_PROJECT="$project" \
FORGELET_E2E_HOST_URL="$host_url" \
FORGELET_E2E_HOST_TOKEN_FILE="$token_file" \
python3 - "$meta" <<'PY'
import json
import pathlib
import sys
import urllib.parse
import urllib.request

meta_path = pathlib.Path(sys.argv[1])
config = json.loads(pathlib.Path(
    __import__("os").environ["MATRIX_BRIDGE_CONFIG"]).read_text())
family = pathlib.Path(__import__("os").environ["FORGELET_E2E_FAMILY_ROOT"])
project = __import__("os").environ["FORGELET_E2E_PROJECT"]
host = __import__("os").environ["FORGELET_E2E_HOST_URL"].rstrip("/")
host_token = pathlib.Path(
    __import__("os").environ["FORGELET_E2E_HOST_TOKEN_FILE"]).read_text().strip()

state_path = pathlib.Path(config["state_dir"]) / "bridge-state.json"
state = json.loads(state_path.read_text())
record = state["forges"][str(family)]["projects"][project]
room = record["room_id"]

base = config["homeserver_url"].rstrip("/")


def request(path, method="GET", body=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(base + path, data=data, headers=headers,
                                 method=method)
    with urllib.request.urlopen(req, timeout=20) as response:
        return json.load(response)


if config.get("access_token"):
    token = config["access_token"]
else:
    login = request("/_matrix/client/v3/login", "POST", {
        "type": "m.login.password",
        "identifier": {"type": "m.id.user", "user": config["user_id"]},
        "password": config["password"],
        "initial_device_display_name": "forgelet bridge project-removal e2e",
    })
    token = login["access_token"]

encoded = urllib.parse.quote(room, safe="")
try:
    messages = request(
        f"/_matrix/client/v3/rooms/{encoded}/messages?dir=b&limit=1",
        token=token)
    event = messages.get("chunk", [{}])[0].get("event_id")
except Exception:
    event = None

if not event:
    sent = request(
        f"/_matrix/client/v3/rooms/{encoded}/send/m.room.message/"
        "forgelet-project-removal-e2e",
        "PUT",
        {"msgtype": "m.text", "body": "project removal history evidence"},
        token,
    )
    event = sent["event_id"]

host_state_req = urllib.request.Request(
    f"{host}/forges/{family.name.removesuffix('-forge')}/api/state",
    headers={"Authorization": "Bearer " + host_token},
)
with urllib.request.urlopen(host_state_req, timeout=30) as response:
    host_state = json.load(response)
target = next((entry for entry in host_state.get("projects", [])
               if entry.get("name") == project), None)
if not target:
    raise SystemExit("project-removal-e2e: host does not name the target project")
dashboard_url = target.get("dashboard_url") or ""
if not dashboard_url:
    raise SystemExit("project-removal-e2e: host names no target runtime")

meta_path.write_text(json.dumps({
    "matrix_token": token,
    "room_id": room,
    "history_event": event,
    "dashboard_url": dashboard_url,
}))
meta_path.chmod(0o600)
PY

if [[ "${FORGELET_E2E_DRY_RUN:-}" == "1" ]]; then
  echo "project-removal-e2e: preflight, runtime, room and history evidence collected"
  exit 0
fi

FORGELET_E2E_HOST_URL="$host_url" \
FORGELET_E2E_HOST_TOKEN_FILE="$token_file" \
FORGELET_E2E_FAMILY_ROOT="$family" \
FORGELET_E2E_PROJECT="$project" \
python3 - "$work/operation-id" <<'PY'
import json
import os
import pathlib
import sys
import urllib.parse
import urllib.request

host = os.environ["FORGELET_E2E_HOST_URL"].rstrip("/")
token = pathlib.Path(os.environ["FORGELET_E2E_HOST_TOKEN_FILE"]).read_text().strip()
slug = pathlib.Path(
    os.environ["FORGELET_E2E_FAMILY_ROOT"]).name.removesuffix("-forge")
project = urllib.parse.quote(os.environ["FORGELET_E2E_PROJECT"], safe="")
req = urllib.request.Request(
    f"{host}/api/forges/{slug}/projects/{project}",
    headers={"Authorization": "Bearer " + token},
    method="DELETE",
)
with urllib.request.urlopen(req, timeout=30) as response:
    operation = json.load(response)
pathlib.Path(sys.argv[1]).write_text(operation["id"])
PY
operation_id="$(cat "$work/operation-id")"

FORGELET_E2E_HOST_URL="$host_url" \
FORGELET_E2E_HOST_TOKEN_FILE="$token_file" \
FORGELET_E2E_FAMILY_ROOT="$family" \
FORGELET_E2E_PROJECT="$project" \
FORGELET_E2E_SIBLING_PROJECT="$sibling" \
python3 - "$work/operation-id" "$work/result.json" <<'PY'
import json
import os
import pathlib
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

host = os.environ["FORGELET_E2E_HOST_URL"].rstrip("/")
token = pathlib.Path(os.environ["FORGELET_E2E_HOST_TOKEN_FILE"]).read_text().strip()
operation_id = pathlib.Path(sys.argv[1]).read_text()
output = pathlib.Path(sys.argv[2])
headers = {"Authorization": "Bearer " + token}


def get_json(path):
    req = urllib.request.Request(host + path, headers=headers)
    with urllib.request.urlopen(req, timeout=30) as response:
        return json.load(response)


seen = []
deadline = time.time() + 300
operation = None
while time.time() < deadline:
    operation = get_json("/api/operations/" + urllib.parse.quote(operation_id, safe=""))
    stage = operation.get("stage") or ""
    if stage and (not seen or seen[-1] != stage):
        seen.append(stage)
    if operation.get("status") in {"completed", "failed"}:
        break
    time.sleep(0.25)
if operation is None or operation.get("status") != "completed":
    error = (operation or {}).get("error") or {}
    code = error.get("code") if isinstance(error, dict) else ""
    print("project-removal-e2e: host operation did not complete")
    if code:
        print("project-removal-e2e: host error " + str(code))
    raise SystemExit(1)
if seen and seen[-1] != "completed":
    raise SystemExit("project-removal-e2e: terminal stage was not completed")

output.write_text(json.dumps({"operation": operation, "stages": seen}))
print("project-removal-e2e: host operation completed")
print("project-removal-e2e: stages seen: " + ", ".join(seen))
PY

MATRIX_BRIDGE_CONFIG="$bridge_config" \
FORGELET_E2E_FAMILY_ROOT="$family" \
FORGELET_E2E_PROJECT="$project" \
FORGELET_E2E_SIBLING_PROJECT="$sibling" \
FORGELET_E2E_HOST_URL="$host_url" \
FORGELET_E2E_HOST_TOKEN_FILE="$token_file" \
python3 - "$meta" "$project_dir" "$marker_name" "$marker_value" <<'PY'
import json
import os
import pathlib
import socket
import sys
import urllib.parse
import urllib.request

meta = json.loads(pathlib.Path(sys.argv[1]).read_text())
project_dir = pathlib.Path(sys.argv[2])
marker_name = sys.argv[3]
marker_value = sys.argv[4]
family = pathlib.Path(os.environ["FORGELET_E2E_FAMILY_ROOT"])
project = os.environ["FORGELET_E2E_PROJECT"]
sibling = os.environ["FORGELET_E2E_SIBLING_PROJECT"]

if project_dir.exists():
    raise SystemExit("project-removal-e2e: the project directory is still in the Family")

open_projects = family / ".swarmforge" / "open-projects"
if open_projects.exists() and project in open_projects.read_text().splitlines():
    raise SystemExit("project-removal-e2e: the project is still in open-projects")

found = None
trash = pathlib.Path.home() / ".Trash"
if trash.is_dir():
    for candidate in trash.iterdir():
        if not candidate.is_dir() or not candidate.name.startswith(project):
            continue
        marker = candidate / marker_name
        if marker.is_file() and marker.read_text().strip() == marker_value:
            found = candidate
            break
if found is None:
    raise SystemExit("project-removal-e2e: the project was not found in macOS Trash")

runtime = urllib.parse.urlparse(meta["dashboard_url"])
if runtime.scheme not in {"http", "https"} or runtime.hostname is None:
    raise SystemExit("project-removal-e2e: the recorded runtime endpoint is invalid")
try:
    socket.create_connection(
        (runtime.hostname, runtime.port or (443 if runtime.scheme == "https" else 80)),
        timeout=0.5,
    ).close()
except OSError:
    pass
else:
    raise SystemExit("project-removal-e2e: the project runtime is still running")

config = json.loads(pathlib.Path(
    os.environ["MATRIX_BRIDGE_CONFIG"]).read_text())
state = json.loads((pathlib.Path(config["state_dir"]) /
                    "bridge-state.json").read_text())
root = str(family)
projects = state["forges"][root]["projects"]
if project in projects:
    raise SystemExit("project-removal-e2e: bridge state still records the target project")
if sibling not in projects or not projects[sibling].get("room_id"):
    raise SystemExit("project-removal-e2e: bridge state lost the sibling project")

host = os.environ["FORGELET_E2E_HOST_URL"].rstrip("/")
host_token = pathlib.Path(
    os.environ["FORGELET_E2E_HOST_TOKEN_FILE"]).read_text().strip()
slug = family.name.removesuffix("-forge")
req = urllib.request.Request(
    f"{host}/forges/{slug}/api/state",
    headers={"Authorization": "Bearer " + host_token},
)
with urllib.request.urlopen(req, timeout=30) as response:
    refreshed = json.load(response)
names = {entry.get("name") for entry in refreshed.get("projects", [])}
if project in names:
    raise SystemExit("project-removal-e2e: the authoritative refresh still names the project")
if sibling not in names:
    raise SystemExit("project-removal-e2e: the authoritative refresh lost the sibling")

base = config["homeserver_url"].rstrip("/")
encoded = urllib.parse.quote(meta["room_id"], safe="")
event = urllib.parse.quote(meta["history_event"], safe="")
req = urllib.request.Request(
    f"{base}/_matrix/client/v3/rooms/{encoded}/event/{event}",
    headers={"Authorization": "Bearer " + meta["matrix_token"]},
)
try:
    with urllib.request.urlopen(req, timeout=20) as response:
        if response.status != 200:
            raise SystemExit("project-removal-e2e: Matrix history was not readable")
except Exception:
    req = urllib.request.Request(
        f"{base}/_matrix/client/v3/rooms/{encoded}/messages?dir=b&limit=20",
        headers={"Authorization": "Bearer " + meta["matrix_token"]},
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as response:
            messages = json.load(response)
    except Exception:
        raise SystemExit("project-removal-e2e: Matrix history was not readable")
    if meta["history_event"] not in {
            item.get("event_id") for item in messages.get("chunk", [])}:
        raise SystemExit("project-removal-e2e: Matrix history evidence was lost")

print("project-removal-e2e: runtime, Trash, refresh, bridge state and Matrix history verified")
PY
