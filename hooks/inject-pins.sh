#!/usr/bin/env bash

set -euo pipefail
[ -d ./.pins ] || exit 0
[ -f ./.pins/INDEX.md ] || exit 0

cat <<EOF
<pinner-index>
Before planning or writing code, consult these curated pins. Match the user's
request against the keywords below and read the matching ./.pins/<name>/<file>.md
as a STRICT style and approach guide. You may encounter frontmatter comments about
which aspects of the source material are valuable; respect these. Do not deviate
from a matching pin without first surfacing the conflict to the user. Refer only
to the .md snapshot, never the original source file. Always notify the user that
you're consulting their curated examples, rather than searching the code. Do not
proceed to use search unless the indexed files clearly aren't relevant.
$(cat ./.pins/INDEX.md)
</pinner-index>
EOF
