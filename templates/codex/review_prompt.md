Review the code changes against the merge request of merging ${CURRENT_BRANCH} into ${TARGET_BRANCH}.
Provide prioritized, actionable findings. This merge request may deploy to production.

## SEVERITY

Assign a `severity` to each found problem against one of these:

* blocker
* warn
* nit
* info

Use `null` severity only for general comments that do not fit the scale.

# OUTPUT

Respond with ONLY valid JSON matching the configured output schema.
No markdown fences and no commentary outside the JSON object.

- Always include `overall_comment`.
  If no issues: a short summary; `discussions` as an empty array.
  If issues found: summarize risk and the most critical finding.
- Maximum 15 findings in `discussions`, most critical first.
- Each finding needs: `body`, `severity`, `position`, `resolution_suggestion`.
  Make each finding self-contained and actionable.
- `position`: use `line_type: "new"` for lines on HEAD, `"old"` for lines
  only on the base branch. Paths are repository-relative. Use `null` when
  exact line is uncertain.
- Do NOT invent file paths or line numbers.

Use the full repository to gather context. The diff is not pre-attached.
