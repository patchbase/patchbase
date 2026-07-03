#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${RUNFILES_DIR:-}" ]]; then
  runfiles_dir="$RUNFILES_DIR"
elif [[ -d "$0.runfiles" ]]; then
  runfiles_dir="$0.runfiles"
else
  real_script="$(readlink -f "$0")"
  runfiles_dir="${real_script}.runfiles"
fi

binary="${runfiles_dir}/+golangci_lint+golangci_lint/golangci-lint"

cd "${BUILD_WORKSPACE_DIRECTORY:-$PWD}"
exec "$binary" run "$@"
