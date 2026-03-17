#!/usr/bin/env bash
set -euo pipefail

ot_die() {
  echo "$1" >&2
  exit 1
}

ot_require_common_args() {
  local target="$1"
  local scope="$2"
  local display="$3"
  local workspace_root="$4"

  if [[ -z "$target" ]]; then
    ot_die "--target is required"
  fi
  if [[ -z "$scope" ]]; then
    ot_die "--scope is required"
  fi
  if [[ "$scope" != "inside" && "$scope" != "outside" ]]; then
    ot_die "invalid scope: $scope"
  fi
  if [[ -z "$display" ]]; then
    ot_die "--display is required"
  fi
  if [[ "$display" != "workspace-relative" && "$display" != "absolute" ]]; then
    ot_die "invalid display: $display"
  fi
  if [[ -z "$workspace_root" ]]; then
    ot_die "--workspace-root is required"
  fi
}

ot_path_contains_hidden_segment() {
  local path="$1"
  local trimmed="${path#/}"
  local old_ifs="$IFS"
  local segment=""

  IFS='/'
  # shellcheck disable=SC2086
  set -- $trimmed
  IFS="$old_ifs"

  for segment in "$@"; do
    if [[ -n "$segment" && "$segment" != "." && "$segment" != ".." && "$segment" == .* ]]; then
      return 0
    fi
  done
  return 1
}

ot_reject_hidden_external_target() {
  local scope="$1"
  local target="$2"

  if [[ "$scope" == "outside" ]] && ot_path_contains_hidden_segment "$target"; then
    ot_die "hidden paths are not allowed outside the workspace: $target"
  fi
}

ot_display_path() {
  local path="$1"
  local display="$2"
  local workspace_root="$3"

  if [[ "$display" == "absolute" ]]; then
    printf '%s\n' "$path"
    return
  fi

  case "$path" in
    "$workspace_root")
      printf '.\n'
      ;;
    "$workspace_root"/*)
      printf '%s\n' "${path#"$workspace_root"/}"
      ;;
    *)
      printf '%s\n' "$path"
      ;;
  esac
}

ot_rewrite_ls_line() {
  local line="$1"
  local actual="$2"
  local shown="$3"

  case "$line" in
    *" $actual")
      printf '%s %s\n' "${line%" $actual"}" "$shown"
      ;;
    "$actual")
      printf '%s\n' "$shown"
      ;;
    *)
      printf '%s\n' "$line"
      ;;
  esac
}

ot_rewrite_rg_line() {
  local line="$1"
  local display="$2"
  local workspace_root="$3"

  if [[ "$display" == "absolute" ]]; then
    printf '%s\n' "$line"
    return
  fi

  case "$line" in
    "$workspace_root"/*)
      printf '%s\n' "${line#"$workspace_root"/}"
      ;;
    *)
      printf '%s\n' "$line"
      ;;
  esac
}
