#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=/dev/null
. "$script_dir/common.sh"

target=""
scope=""
display=""
workspace_root=""
name=""
content=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      target="${2:-}"
      shift 2
      ;;
    --scope)
      scope="${2:-}"
      shift 2
      ;;
    --display)
      display="${2:-}"
      shift 2
      ;;
    --workspace-root)
      workspace_root="${2:-}"
      shift 2
      ;;
    --name)
      name="${2:-}"
      shift 2
      ;;
    --content)
      content="${2:-}"
      shift 2
      ;;
    *)
      ot_die "unknown arg: $1"
      ;;
  esac
done

ot_require_common_args "$target" "$scope" "$display" "$workspace_root"
ot_reject_hidden_external_target "$scope" "$target"

if [[ ! -e "$target" ]]; then
  ot_die "path not found: $target"
fi
if [[ -z "$name" && -z "$content" ]]; then
  ot_die "ot search requires --name or --content"
fi

print_name_matches() {
  local search_root="$1"
  local pattern="$2"

  if [[ -f "$search_root" ]]; then
    if [[ "$(basename "$search_root")" == $pattern ]]; then
      ot_display_path "$search_root" "$display" "$workspace_root"
    fi
    return
  fi

  if [[ "$scope" == "inside" ]]; then
    while IFS= read -r entry; do
      ot_display_path "$entry" "$display" "$workspace_root"
    done < <(find "$search_root" -name "$pattern" -print)
  else
    while IFS= read -r entry; do
      ot_display_path "$entry" "$display" "$workspace_root"
    done < <(find "$search_root" \( -name '.*' -o -path '*/.*' \) -prune -o -name "$pattern" -print)
  fi
}

run_content_search() {
  local pattern="$1"
  shift

  local -a cmd
  local rg_bin="${OT_RG_BIN:-rg}"
  local output=""
  local status=0
  local line=""

  cmd=("$rg_bin" --line-number --color never --no-heading)
  if [[ "$scope" == "inside" ]]; then
    cmd+=(--hidden)
  fi
  cmd+=(-- "$pattern")
  cmd+=("$@")

  set +e
  output="$("${cmd[@]}")"
  status=$?
  set -e

  if [[ $status -eq 1 ]]; then
    return 0
  fi
  if [[ $status -ne 0 ]]; then
    exit "$status"
  fi

  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    ot_rewrite_rg_line "$line" "$display" "$workspace_root"
  done <<< "$output"
}

if [[ -n "$name" && -z "$content" ]]; then
  print_name_matches "$target" "$name"
  exit 0
fi

if [[ -z "$name" && -n "$content" ]]; then
  run_content_search "$content" "$target"
  exit 0
fi

matches=()
if [[ -f "$target" ]]; then
  if [[ "$(basename "$target")" == $name ]]; then
    matches+=("$target")
  fi
else
  if [[ "$scope" == "inside" ]]; then
    while IFS= read -r -d '' entry; do
      matches+=("$entry")
    done < <(find "$target" -type f -name "$name" -print0)
  else
    while IFS= read -r -d '' entry; do
      matches+=("$entry")
    done < <(find "$target" \( -name '.*' -o -path '*/.*' \) -prune -o -type f -name "$name" -print0)
  fi
fi

if [[ ${#matches[@]} -eq 0 ]]; then
  exit 0
fi

run_content_search "$content" "${matches[@]}"
