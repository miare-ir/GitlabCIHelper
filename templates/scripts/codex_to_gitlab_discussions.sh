#!/usr/bin/env bash

set -euo pipefail

OUTPUT_FILE="${1:-codex_review.json}"

if [[ ! -s "${OUTPUT_FILE}" ]]; then
  echo "Codex review output '${OUTPUT_FILE}' is missing or empty; skipping discussion creation." >&2
  exit 0
fi

for dependency in curl jq python3; do
  if ! command -v "${dependency}" >/dev/null 2>&1; then
    echo "Required dependency '${dependency}' is not installed." >&2
    exit 1
  fi
done

: "${CI_PROJECT_ID:?CI_PROJECT_ID is required}"
TOKEN="${GITLAB_CI_HELPER_TOKEN:-${PRIVATE_TOKEN:-}}"
: "${TOKEN:?GITLAB_CI_HELPER_TOKEN (or PRIVATE_TOKEN) is required}"

API_URL="${CI_API_V4_URL:-${CI_PROJECT_URL%/}/api/v4}"
CURRENT_BRANCH="${CI_COMMIT_REF_NAME:-}"
MR_IID="${CI_MERGE_REQUEST_IID:-}"

api_request() {
  local attempts=4
  local delay=1
  local attempt
  local output
  local status
  for ((attempt=1; attempt<=attempts; attempt++)); do
    set +e
    output=$(curl --fail-with-body -sS "$@" 2>&1)
    status=$?
    set -e

    if [[ "${status}" -eq 0 ]]; then
      printf '%s' "${output}"
      return 0
    fi

    if [[ "${attempt}" -eq "${attempts}" ]]; then
      echo "${output}" >&2
      return "${status}"
    fi

    sleep "${delay}"
    delay=$((delay * 2))
  done
}

if [[ -z "${MR_IID}" ]]; then
  if [[ -z "${CURRENT_BRANCH}" ]]; then
    echo "Unable to determine merge request IID: CI_COMMIT_REF_NAME is missing." >&2
    exit 1
  fi
  BRANCH_ENCODED=$(python3 -c 'import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1]))' "${CURRENT_BRANCH}")
  MR_IID=$(api_request --header "PRIVATE-TOKEN: ${TOKEN}" \
    "${API_URL}/projects/${CI_PROJECT_ID}/merge_requests?state=opened&source_branch=${BRANCH_ENCODED}" \
    | jq -r '.[0].iid // empty')
fi

if [[ -z "${MR_IID}" ]]; then
  echo "No open merge request found for branch '${CURRENT_BRANCH}'. Skipping." >&2
  exit 0
fi

MR_INFO=$(api_request --header "PRIVATE-TOKEN: ${TOKEN}" \
  "${API_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MR_IID}")

BASE_SHA=$(echo "${MR_INFO}" | jq -r '.diff_refs.base_sha // empty')
START_SHA=$(echo "${MR_INFO}" | jq -r '.diff_refs.start_sha // empty')
HEAD_SHA=$(echo "${MR_INFO}" | jq -r '.diff_refs.head_sha // empty')

if [[ -z "${BASE_SHA}" ]]; then
  echo "Unable to retrieve diff refs for merge request !${MR_IID}." >&2
  exit 1
fi

MARKER="<!-- gitlab-ci-helper-codex-review -->"

post_general_note() {
  local message="$1"
  local body
  body=$(python3 -c 'import sys; print(sys.stdin.read().strip())' <<<"${message}")
  local payload
  payload=$(jq -n --arg body "${body}" --arg marker "${MARKER}" '{ body: ($body + "\n\n" + $marker) }')
  api_request --request POST \
    --header "PRIVATE-TOKEN: ${TOKEN}" \
    --header "Content-Type: application/json" \
    --data "${payload}" \
    "${API_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MR_IID}/notes" >/dev/null
}

post_discussion() {
  local message="$1"
  local path="$2"
  local line="$3"
  local line_type="$4"

  local body
  body=$(python3 -c 'import sys; print(sys.stdin.read().strip())' <<<"${message}")

  if [[ -z "${line_type}" || "${line_type}" == "null" ]]; then
    line_type="new"
  fi

  local payload
  payload=$(
    jq -n \
      --arg body "${body}" \
      --arg marker "${MARKER}" \
      --arg base "${BASE_SHA}" \
      --arg start "${START_SHA}" \
      --arg head "${HEAD_SHA}" \
      --arg path "${path}" \
      --arg type "${line_type}" \
      --argjson line "${line}" \
      '{
        body: ($body + "\n\n" + $marker),
        position: (
          if $type == "old" then
            {
              position_type: "text",
              base_sha: $base,
              start_sha: $start,
              head_sha: $head,
              old_path: $path
            } + (if $line != null then { old_line: $line } else {} end)
          else
            {
              position_type: "text",
              base_sha: $base,
              start_sha: $start,
              head_sha: $head,
              new_path: $path
            } + (if $line != null then { new_line: $line } else {} end)
          end
        )
      }'
  )

  api_request --request POST \
    --header "PRIVATE-TOKEN: ${TOKEN}" \
    --header "Content-Type: application/json" \
    --data "${payload}" \
    "${API_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MR_IID}/discussions" >/dev/null
}

OVERALL_COMMENT=$(jq -r '.overall_comment // empty' "${OUTPUT_FILE}")
if [[ -n "${OVERALL_COMMENT}" ]]; then
  OVERALL_COMMENT=$(printf '%b' "${OVERALL_COMMENT//%/%%}")
  post_general_note "${OVERALL_COMMENT}"
fi

DISCUSSION_COUNT=$(jq '.discussions | length' "${OUTPUT_FILE}")
if [[ "${DISCUSSION_COUNT}" -eq 0 ]]; then
  echo "No Codex discussions to post." >&2
  exit 0
fi

jq -c '.discussions[]' "${OUTPUT_FILE}" | while read -r discussion; do
  BODY=$(echo "${discussion}" | jq -r '.body')
  SEVERITY=$(echo "${discussion}" | jq -r '.severity // empty')
  SUGGESTION=$(echo "${discussion}" | jq -r '.resolution_suggestion // empty')
  PATH_VALUE=$(echo "${discussion}" | jq -r '.position.path // empty')
  LINE_VALUE=$(echo "${discussion}" | jq '.position.line // null')
  LINE_TYPE=$(echo "${discussion}" | jq -r '.position.line_type // empty')

  if [[ -z "${BODY}" || "${BODY}" == "null" ]]; then
    continue
  fi

  BODY=$(printf '%b' "${BODY//%/%%}")
  SUGGESTION=$(printf '%b' "${SUGGESTION//%/%%}")

  COMMENT_BODY="${BODY}"
  if [[ -n "${SEVERITY}" ]]; then
    printf -v COMMENT_BODY "**Severity:** %s\n\n%s" "${SEVERITY}" "${COMMENT_BODY}"
  fi
  if [[ -n "${SUGGESTION}" ]]; then
    printf -v COMMENT_BODY "%s\n\n**Suggested fix:** %s" "${COMMENT_BODY}" "${SUGGESTION}"
  fi

  if [[ -z "${PATH_VALUE}" || "${LINE_VALUE}" == "null" ]]; then
    post_general_note "${COMMENT_BODY}"
  else
    post_discussion "${COMMENT_BODY}" "${PATH_VALUE}" "${LINE_VALUE}" "${LINE_TYPE}"
  fi
done
