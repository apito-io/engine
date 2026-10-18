#!/usr/bin/env bash
# Write the license manifest for a new public release.
# Usage: scripts/release-license-manifest.sh <tag> <availability-date-YYYY-MM-DD> <out-file>
set -euo pipefail

tag="${1:?tag}"
availability="${2:?availability date YYYY-MM-DD}"
out="${3:?output file}"

python3 - "$tag" "$availability" "$out" <<'PY'
import datetime, json, sys
tag, availability, out = sys.argv[1:]
start = datetime.date.fromisoformat(availability)
doc = {
    "product": "apito-engine",
    "tag": tag,
    "spdx": "AGPL-3.0-only",
    "availability_date": start.isoformat(),
    "note": "Releases after v1.8.30 are AGPL-3.0-only. Tags v1.8.26 through v1.8.30 remain FSL-1.1-ALv2. Tags through v1.8.25 remain Apache-2.0.",
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
print(start.isoformat())
PY
