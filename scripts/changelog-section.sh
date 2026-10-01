#!/usr/bin/env bash
# =============================================================================
# changelog-section.sh  —  print the CHANGELOG.md section of one release
#
# Usage:   bash scripts/changelog-section.sh <version> [CHANGELOG.md]
#          (version with or without the leading v: 1.5.0 or v1.5.0)
#
# Prints the body under "## [<version>]" up to the next "## [" heading, without
# surrounding blank lines. Fails when the section is missing or empty, so a tag
# whose changelog was not moved under its version gets no release.
# =============================================================================
set -euo pipefail

version="${1:?usage: changelog-section.sh <version> [CHANGELOG.md]}"
version="${version#v}"
changelog="${2:-CHANGELOG.md}"

body=$(awk -v v="$version" '
  index($0, "## [" v "]") == 1 { found = 1; next }
  found && /^## \[/            { exit }
  found && /^\[[^]]+\]: /      { exit }
  found                        { print }
' "$changelog" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}')

if [ -z "$body" ]; then
  echo "changelog-section: no \"## [${version}]\" section with content in ${changelog}" >&2
  exit 1
fi
printf '%s\n' "$body"
