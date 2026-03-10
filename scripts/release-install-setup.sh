#!/usr/bin/env bash
set -euo pipefail

repo="miare-ir/GitlabCIHelper"
binary_url="https://github.com/${repo}/releases/latest/download/gitlab-ci-helper"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required but not installed." >&2
  exit 1
fi

if ! command -v mktemp >/dev/null 2>&1; then
  echo "mktemp is required but not installed." >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

binary_path="${tmpdir}/gitlab-ci-helper"

echo "Downloading gitlab-ci-helper..."
curl -fsSL "${binary_url}" -o "${binary_path}"
chmod +x "${binary_path}"

if [ "$#" -eq 0 ]; then
  set -- setup
fi

echo "Running: gitlab-ci-helper $*"
"${binary_path}" "$@"
