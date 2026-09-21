#!/usr/bin/env zsh

terminal_backend_label() {
  echo "Kitty"
}

kitty_remote_control() {
  if [[ -n "${KITTY_LISTEN_ON:-}" ]]; then
    local address="$KITTY_LISTEN_ON"
    # Kitty sometimes reports a listen_on template (e.g. unix:/tmp/kitty.sock)
    # while the live socket is PID-suffixed. Fall back to the suffixed path
    # if the literal one from the env var isn't actually there.
    if [[ "$address" == unix:* && -n "${KITTY_PID:-}" ]]; then
      local socket_path="${address#unix:}"
      if [[ ! -S "$socket_path" && -S "${socket_path}-${KITTY_PID}" ]]; then
        address="unix:${socket_path}-${KITTY_PID}"
      fi
    fi
    kitten @ --to "$address" "$@"
  else
    kitten @ "$@"
  fi
}

terminal_backend_can_open_sessions() {
  if kitty_remote_control ls >/dev/null 2>&1; then
    return 0
  fi

  print -u2 "Kitty remote control is unavailable. Run 'kitten @ ls' in this Kitty shell to diagnose it, then set 'allow_remote_control yes' in kitty.conf."
  return 1
}

terminal_backend_tracks_windows() {
  return 0
}

terminal_window_exists() {
  local window_id="$1"
  [[ -n "$window_id" ]] || return 1

  kitty_remote_control ls --match "id:$window_id" >/dev/null 2>&1
}

terminal_open_session() {
  local session="$1"
  local title="$2"
  local sibling_id="${3:-}"
  local placement_args=()
  local window_id=""
  local theme_file=""

  if [[ -n "$sibling_id" ]]; then
    # Split next to an existing panel from this same batch; kitty resolves
    # --next-to by window id, so this lands in whichever OS window that
    # sibling opened in rather than wherever the caller's focus is.
    placement_args=(--type=window --location=split --next-to "id:$sibling_id")
  else
    # First panel of a batch: open a brand-new OS window instead of
    # splitting into whatever kitty window currently has focus.
    placement_args=(--type=os-window)
  fi

  # Per-project theme: if <project>/.swarmforge/kitty-theme.conf exists, apply
  # it to this window after launch. Kitty colors are per-window, so each panel
  # gets the theme without changing the rest of the kitty instance.
  if [[ -n "${WORKING_DIR:-}" && -f "$WORKING_DIR/.swarmforge/kitty-theme.conf" ]]; then
    theme_file="$WORKING_DIR/.swarmforge/kitty-theme.conf"
  fi

  window_id="$(kitty_remote_control launch \
    "${placement_args[@]}" \
    --cwd "$WORKING_DIR" \
    --title "$title" \
    tmux -S "$TMUX_SOCKET" attach-session -t "$session")"

  if [[ -n "$window_id" && -n "$theme_file" ]]; then
    kitty_remote_control set-colors --match "id:$window_id" "$theme_file" >/dev/null 2>&1 || true
  fi

  print -- "$window_id"
}

terminal_close_window() {
  local window_id="$1"
  [[ -n "$window_id" ]] || return 0

  kitty_remote_control close-window --match "id:$window_id" >/dev/null 2>&1 || true
}
