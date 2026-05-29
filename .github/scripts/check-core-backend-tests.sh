#!/usr/bin/env bash
set -euo pipefail

base_ref="${1:-origin/main}"
head_ref="${2:-HEAD}"

changed_files="$(git diff --name-only "${base_ref}...${head_ref}" --)"

if [ -z "${changed_files}" ]; then
  exit 0
fi

backend_changes="$(
  printf '%s\n' "${changed_files}" |
    grep -E '^(apps/core/(cmd|internal)/.*\.(go|sql)|apps/core/main\.go|apps/core/go\.(mod|sum))$' |
    grep -Ev '(_test\.go$)' || true
)"

if [ -z "${backend_changes}" ]; then
  exit 0
fi

test_changes="$(
  printf '%s\n' "${changed_files}" |
    grep -E '^apps/core/.*_test\.go$' || true
)"

if [ -n "${test_changes}" ]; then
  exit 0
fi

pr_body="${PR_BODY:-}"
if printf '%s\n' "${pr_body}" | grep -Eq 'No Core backend test changes because:[[:space:]]*[[:graph:]]'; then
  echo "Core backend files changed without Core test files, but the PR includes an explicit test rationale."
  exit 0
fi

cat <<'EOF'
Core backend files changed without Core test changes.

Add focused Core backend tests, or fill in the PR template line:

No Core backend test changes because: <rationale>

Backend files:
EOF
printf '%s\n' "${backend_changes}"
exit 1
