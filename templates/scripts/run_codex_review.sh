#!/usr/bin/env bash

set -euo pipefail

for dependency in codex curl jq envsubst git python3; do
  if ! command -v "${dependency}" >/dev/null 2>&1; then
    echo "Required dependency '${dependency}' is not installed." >&2
    exit 1
  fi
done

: "${GITLAB_CI_HELPER_TOKEN:?GITLAB_CI_HELPER_TOKEN is required}"
: "${GITLAB_CI_HELPER_CODEX_AUTH:?GITLAB_CI_HELPER_CODEX_AUTH is required}"

TARGET_BRANCH="${CI_MERGE_REQUEST_TARGET_BRANCH_NAME:-${CI_DEFAULT_BRANCH:-master}}"
HELPER_ROOT="${GITLAB_CI_HELPER_ROOT_DIR:-.gitlab-ci-helper}"

retry_cmd() {
  local attempts=4
  local delay=1
  local attempt
  for ((attempt=1; attempt<=attempts; attempt++)); do
    if "$@"; then
      return 0
    fi
    if [[ "${attempt}" -eq "${attempts}" ]]; then
      return 1
    fi
    sleep "${delay}"
    delay=$((delay * 2))
  done
}

mkdir -p ~/.codex/
cp "${GITLAB_CI_HELPER_CODEX_AUTH}" ~/.codex/auth.json
codex login status

if ! retry_cmd git fetch origin "${TARGET_BRANCH}" --depth=200; then
  retry_cmd git fetch origin "${TARGET_BRANCH}"
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

PROMPT_TEMPLATE="${TMP_DIR}/review_prompt.md"
if [[ -n "${GITLAB_CI_HELPER_CODEX_PROMPT_PATH:-}" && -f "${GITLAB_CI_HELPER_CODEX_PROMPT_PATH}" ]]; then
  cp "${GITLAB_CI_HELPER_CODEX_PROMPT_PATH}" "${PROMPT_TEMPLATE}"
else
  DEFAULT_PROMPT="${HELPER_ROOT}/templates/codex/review_prompt.md"
  if [[ ! -f "${DEFAULT_PROMPT}" ]]; then
    echo "Built-in prompt template is missing: ${DEFAULT_PROMPT}" >&2
    exit 1
  fi
  cp "${DEFAULT_PROMPT}" "${PROMPT_TEMPLATE}"
fi

SCHEMA_PATH="${TMP_DIR}/review_output_schema.json"
if [[ -n "${GITLAB_CI_HELPER_CODEX_SCHEMA_PATH:-}" && -f "${GITLAB_CI_HELPER_CODEX_SCHEMA_PATH}" ]]; then
  cp "${GITLAB_CI_HELPER_CODEX_SCHEMA_PATH}" "${SCHEMA_PATH}"
else
  DEFAULT_SCHEMA="${HELPER_ROOT}/templates/codex/review_output_schema.json"
  if [[ ! -f "${DEFAULT_SCHEMA}" ]]; then
    echo "Built-in review schema is missing: ${DEFAULT_SCHEMA}" >&2
    exit 1
  fi
  cp "${DEFAULT_SCHEMA}" "${SCHEMA_PATH}"
fi

env CURRENT_BRANCH="${CI_COMMIT_REF_NAME}" TARGET_BRANCH="${TARGET_BRANCH}" envsubst < "${PROMPT_TEMPLATE}" > "${TMP_DIR}/codex_prompt.txt"

set -o pipefail
codex exec --json --model="${GITLAB_CI_HELPER_CODEX_REVIEW_MODEL:-gpt-5.3-codex}" \
  --config model_reasoning_effort="high" \
  --output-schema="${SCHEMA_PATH}" \
  - \
  < "${TMP_DIR}/codex_prompt.txt" \
  > "${TMP_DIR}/codex_raw.jsonl"

jq -rs '[.[] | select(.type=="item.completed" and .item.type=="agent_message") | .item.text] | last' "${TMP_DIR}/codex_raw.jsonl" > "${TMP_DIR}/codex_review.json"
if [[ ! -s "${TMP_DIR}/codex_review.json" ]]; then
  echo "Codex did not produce a final agent message. Raw events:" >&2
  cat "${TMP_DIR}/codex_raw.jsonl" >&2 || true
  exit 1
fi

POST_SCRIPT="${HELPER_ROOT}/templates/scripts/codex_to_gitlab_discussions.sh"
if [[ ! -f "${POST_SCRIPT}" ]]; then
  echo "Built-in discussion publisher is missing: ${POST_SCRIPT}" >&2
  exit 1
fi

bash "${POST_SCRIPT}" "${TMP_DIR}/codex_review.json"
