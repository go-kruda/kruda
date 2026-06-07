#!/usr/bin/env bash
# Report same-runner benchstat regressions above a threshold.
set -euo pipefail

mode="check"
threshold="${BENCHSTAT_REGRESSION_THRESHOLD:-10}"

usage() {
  echo "usage: $0 [--list] [--threshold percent] benchstat.txt" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --list)
      mode="list"
      shift
      ;;
    --threshold)
      if [[ $# -lt 2 ]]; then
        usage
        exit 2
      fi
      threshold="$2"
      shift 2
      ;;
    -*)
      usage
      exit 2
      ;;
    *)
      break
      ;;
  esac
done

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

benchstat_file="$1"
if [[ ! -f "$benchstat_file" ]]; then
  echo "benchstat file not found: $benchstat_file" >&2
  exit 2
fi

regressions_file="$(mktemp)"
trap 'rm -f "$regressions_file"' EXIT

awk -v threshold="$threshold" '
  /sec\/op/ {
    metric = "sec/op"
  }
  /B\/op/ {
    metric = "B/op"
  }
  /allocs\/op/ {
    metric = "allocs/op"
  }
  /^[^[:space:]]/ && $0 ~ /\+[0-9.]+%/ {
    for (i = 1; i <= NF; i++) {
      if ($i ~ /^\+[0-9.]+%$/) {
        change = $i
        gsub(/^\+|%$/, "", change)
        if ((change + 0) > threshold) {
          printf("- `%s` %s %s\n", $1, metric, $i)
        }
      }
    }
  }
' "$benchstat_file" > "$regressions_file"

if [[ ! -s "$regressions_file" ]]; then
  exit 0
fi

cat "$regressions_file"

if [[ "$mode" == "check" ]]; then
  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    echo "::error::Benchmark regression above ${threshold}% detected by same-runner benchstat."
  else
    echo "Benchmark regression above ${threshold}% detected by same-runner benchstat." >&2
  fi
  exit 1
fi
