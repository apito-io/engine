#!/usr/bin/env bash
# Write a Fair Source release manifest. Dates are immutable once published.
# Usage: scripts/release-license-manifest.sh <tag> <availability-date-YYYY-MM-DD> <out-file>
set -euo pipefail

tag="${1:?tag}"
availability="${2:?availability date YYYY-MM-DD}"
out="${3:?output file}"

python3 - "$tag" "$availability" "$out" <<'PY'
import datetime, json, sys
tag, availability, out = sys.argv[1:]
start = datetime.date.fromisoformat(availability)
change = start.replace(year=start.year + 2)
doc = {
    "product": "apito-engine",
    "tag": tag,
    "spdx": "FSL-1.1-ALv2",
    "also_known_as": "FSL-1.1-Apache-2.0",
    "availability_date": start.isoformat(),
    "apache_change_date": change.isoformat(),
    "future_license": "Apache-2.0",
    "note": "Do not describe this release as OSI open source. It converts to Apache-2.0 on apache_change_date.",
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
print(f"{start.isoformat()} -> {change.isoformat()}")
PY
