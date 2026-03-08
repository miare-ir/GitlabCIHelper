#!/usr/bin/env bash

set -euo pipefail

for dependency in curl jq python3; do
  if ! command -v "${dependency}" >/dev/null 2>&1; then
    echo "Required dependency '${dependency}' is not installed." >&2
    exit 1
  fi
done

: "${CI_PROJECT_ID:?CI_PROJECT_ID is required}"
: "${CI_COMMIT_REF_NAME:?CI_COMMIT_REF_NAME is required}"
: "${GITLAB_CI_HELPER_TOKEN:?GITLAB_CI_HELPER_TOKEN is required}"

API_URL="${CI_API_V4_URL:-${CI_PROJECT_URL%/}/api/v4}"
TARGET_BRANCH="${GITLAB_CI_HELPER_TARGET_BRANCH:-}"
MR_TITLE="${GITLAB_CI_HELPER_MR_TITLE:-${CI_COMMIT_REF_NAME}}"
ASSIGN_CREATOR="${GITLAB_CI_HELPER_ASSIGN_CREATOR:-true}"

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

if [[ -z "${TARGET_BRANCH}" ]]; then
  TARGET_BRANCH=$(api_request \
    --header "PRIVATE-TOKEN: ${GITLAB_CI_HELPER_TOKEN}" \
    "${API_URL}/projects/${CI_PROJECT_ID}" \
    | jq -r '.default_branch // empty')
fi

if [[ -z "${TARGET_BRANCH}" ]]; then
  echo "Unable to resolve target branch." >&2
  exit 1
fi

MR_TEMPLATE_PATH="${GITLAB_CI_HELPER_MR_TEMPLATE_PATH:-.gitlab-ci-helper/mr_description.md}"
if [[ ! -f "${MR_TEMPLATE_PATH}" ]]; then
  echo "MR description template file not found at '${MR_TEMPLATE_PATH}'." >&2
  echo "Set GITLAB_CI_HELPER_MR_TEMPLATE_PATH to a valid path or sync .gitlab-ci-helper assets." >&2
  exit 1
fi
DESCRIPTION_TEMPLATE=$(cat "${MR_TEMPLATE_PATH}")

BRANCH_ENCODED=$(python3 -c 'import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1]))' "${CI_COMMIT_REF_NAME}")
LISTMR=$(api_request \
  --header "PRIVATE-TOKEN: ${GITLAB_CI_HELPER_TOKEN}" \
  "${API_URL}/projects/${CI_PROJECT_ID}/merge_requests?state=opened&source_branch=${BRANCH_ENCODED}")

# Keep current behavior: check opened MRs only by source_branch.
COUNTBRANCHES=$(echo "${LISTMR}" | jq --arg source "${CI_COMMIT_REF_NAME}" '[.[] | select(.source_branch == $source)] | length')
if [[ "${COUNTBRANCHES}" -ne 0 ]]; then
  echo "No new merge request opened"
  exit 0
fi

if [[ "${ASSIGN_CREATOR}" == "true" && -n "${GITLAB_USER_ID:-}" ]]; then
  BODY=$(jq -n \
    --arg source_branch "${CI_COMMIT_REF_NAME}" \
    --arg target_branch "${TARGET_BRANCH}" \
    --arg title "${MR_TITLE}" \
    --arg description "${DESCRIPTION_TEMPLATE}" \
    --argjson assignee_id "${GITLAB_USER_ID}" \
    '{
      id: env.CI_PROJECT_ID,
      source_branch: $source_branch,
      target_branch: $target_branch,
      remove_source_branch: true,
      title: $title,
      description: $description,
      assignee_id: $assignee_id
    }')
else
  BODY=$(jq -n \
    --arg source_branch "${CI_COMMIT_REF_NAME}" \
    --arg target_branch "${TARGET_BRANCH}" \
    --arg title "${MR_TITLE}" \
    --arg description "${DESCRIPTION_TEMPLATE}" \
    '{
      id: env.CI_PROJECT_ID,
      source_branch: $source_branch,
      target_branch: $target_branch,
      remove_source_branch: true,
      title: $title,
      description: $description
    }')
fi

MR_RESPONSE=$(api_request \
  --request POST \
  --header "PRIVATE-TOKEN: ${GITLAB_CI_HELPER_TOKEN}" \
  --header "Content-Type: application/json" \
  --data "${BODY}" \
  "${API_URL}/projects/${CI_PROJECT_ID}/merge_requests")

MR_IID=$(echo "${MR_RESPONSE}" | jq -r '.iid // empty')
if [[ -z "${MR_IID}" ]]; then
  echo "Unable to parse merge request IID from response." >&2
  echo "${MR_RESPONSE}" >&2
  exit 1
fi

DISCUSSION_PAYLOAD=$(jq -n --arg body "Review Required" '{ body: $body }')
api_request \
  --request POST \
  --header "PRIVATE-TOKEN: ${GITLAB_CI_HELPER_TOKEN}" \
  --header "Content-Type: application/json" \
  --data "${DISCUSSION_PAYLOAD}" \
  "${API_URL}/projects/${CI_PROJECT_ID}/merge_requests/${MR_IID}/discussions" >/dev/null

echo "Opened a new merge request: from ${CI_COMMIT_REF_NAME} into ${TARGET_BRANCH}"
