#!/usr/bin/env bash

set -euo pipefail
[ -d ./pins ] || exit 0
[ -f ./pins/INDEX.md ] || exit 0

cat <<EOF
<pinner-index>
Before planning or writing code, consult these curated pins. Match the user's
request against the keywords below and read the matching ./pins/<name>/<file>.md
as a STRICT style and approach guide. Do not deviate from a matching pin without
first surfacing the conflict to the user. Refer only to the .md snapshot, never
the original source file.

$(cat ./pins/INDEX.md)
</pinner-index>
EOF
