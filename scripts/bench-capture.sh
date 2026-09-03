#!/usr/bin/env bash
# Capture the Go benchmarks as a TSV carrying the machine conditions the
# numbers were taken under (doctrine R11: a number compared across sessions
# records its conditions, as metadata, never as a gate on taking it).
#
# Usage:
#   scripts/bench-capture.sh [OUT.tsv]   capture (default: a dated file in the
#                                        gitignored working folder's bench tier)
#   scripts/bench-capture.sh --list      print the benchmarked packages
#
# Environment: BENCH_COUNT (default 7), BENCH_TIME (default 1s).
#
# Exits non-zero when `go test` fails or when the scan produced no cells — an
# empty scan is a broken capture, not an empty benchmark set (R1).
set -euo pipefail
# Go prints `84.06 ns/op` with a dot in every locale; awk must read it the same
# way, and this host's default locale uses a decimal comma.
export LC_ALL=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# The single list of benchmarked packages. `make bench` reads it back through
# --list, so the Makefile target and this capture cannot drift apart.
BENCH_PKGS=(
    ./internal/buffer
    ./internal/fuzzy
    ./internal/git
    ./internal/highlight
    ./internal/navigator
    ./internal/benchcontrol
)

if [[ "${1:-}" == "--list" ]]; then
    printf '%s\n' "${BENCH_PKGS[*]}"
    exit 0
fi

COUNT=${BENCH_COUNT:-7}
BENCHTIME=${BENCH_TIME:-1s}
[[ "$COUNT" =~ ^[1-9][0-9]*$ ]] || {
    echo "BENCH_COUNT must be a positive integer (got '$COUNT')" >&2
    exit 2
}

cd "$ROOT"
SHA=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
OUT=${1:-dev-docs/bench/results/$(date -u +%Y-%m-%d)-$SHA.tsv}

# The `#` block below reuses baseline.sh's environment keys (date, commit,
# branch, go, os, arch) and adds what the anchor check reads back: host,
# go_series, and the load the capture ran under. Two load samples, because one
# is not readable on its own: loadavg_start is the load the machine was already
# under when the run began, loadavg_end includes the benchmarks' own load.
write_header() {
    local out=$1 os
    if [[ "$(uname -s)" == Darwin ]]; then
        os="macOS $(sw_vers -productVersion)"
    else
        os=$(uname -sr)
    fi
    {
        echo "# host=$(hostname -s)"
        echo "# arch=$(uname -m)"
        echo "# os=$os"
        echo "# go=$(go env GOVERSION)"
        echo "# go_series=$(go env GOVERSION | sed -E 's/^(go[0-9]+\.[0-9]+).*/\1/')"
        echo "# go_full=$(go version)"
        echo "# commit=$(git rev-parse HEAD 2>/dev/null || echo unknown)"
        echo "# branch=$(git branch --show-current 2>/dev/null || echo unknown)"
        echo "# describe=$(git describe --tags --always 2>/dev/null || echo unknown)"
        echo "# date=$STARTED"
        echo "# loadavg_start=$LOAD_START"
        echo "# loadavg_end=$(loadavg)"
        echo "# count=$COUNT"
        echo "# benchtime=$BENCHTIME"
        printf '#name\tn\tmedian_ns\tmin_ns\tmean_ns\tb_per_op\tallocs_per_op\n'
    } >"$out"
}

# One row per benchmark name, aggregating the -count samples. Median first
# because a heavy-tailed cell's min reports a lucky round rather than a rate;
# min and mean ride along so a distribution shift stays visible (R11).
parse_samples() {
    awk '
    function sortvals(v, c,   i, j, t) {
        for (i = 2; i <= c; i++) {
            t = v[i]
            for (j = i - 1; j >= 1 && v[j] > t; j--) v[j + 1] = v[j]
            v[j + 1] = t
        }
    }
    function median(v, c) {
        return (c % 2) ? v[(c + 1) / 2] : (v[c / 2] + v[c / 2 + 1]) / 2.0
    }
    $1 ~ /^Benchmark/ && $2 ~ /^[0-9]+$/ {
        nsval = ""; boval = 0; aoval = 0
        for (i = 3; i <= NF; i++) {
            if ($i == "ns/op") nsval = $(i - 1)
            else if ($i == "B/op") boval = $(i - 1)
            else if ($i == "allocs/op") aoval = $(i - 1)
        }
        if (nsval == "") next
        name = $1
        if (!(name in count)) order[++names] = name
        c = ++count[name]
        ns[name, c] = nsval + 0
        bo[name, c] = boval + 0
        ao[name, c] = aoval + 0
    }
    END {
        for (j = 1; j <= names; j++) {
            name = order[j]; c = count[name]
            delete sn; delete sb; delete sa
            sum = 0
            for (i = 1; i <= c; i++) {
                sn[i] = ns[name, i]; sb[i] = bo[name, i]; sa[i] = ao[name, i]
                sum += sn[i]
            }
            sortvals(sn, c); sortvals(sb, c); sortvals(sa, c)
            printf "%s\t%d\t%.6g\t%.6g\t%.6g\t%.6g\t%.6g\n", \
                name, c, median(sn, c), sn[1], sum / c, median(sb, c), median(sa, c)
        }
    }
    ' "$1"
}

loadavg() { uptime | sed -E 's/.*load averages?:[[:space:]]*//'; }

STARTED=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LOAD_START=$(loadavg)

RAW=$(mktemp "${TMPDIR:-/tmp}/zephyr-bench.XXXXXX")
trap 'rm -f "$RAW"' EXIT

echo "bench-capture: ${#BENCH_PKGS[@]} package(s), -count=$COUNT -benchtime=$BENCHTIME"
rc=0
go test "${BENCH_PKGS[@]}" -run '^$' -bench=. -benchmem \
    -count="$COUNT" -benchtime="$BENCHTIME" >"$RAW" 2>&1 || rc=$?
if ((rc != 0)); then
    echo "FAIL: go test exited $rc — no capture written" >&2
    tail -40 "$RAW" >&2
    exit 1
fi

mkdir -p "$(dirname "$OUT")"
write_header "$OUT"
parse_samples "$RAW" >>"$OUT"

cells=$(grep -cv '^#' "$OUT" || true)
if [[ "${cells:-0}" -eq 0 ]]; then
    echo "FAIL: go test succeeded but the scan produced 0 benchmark cells" >&2
    echo "  an empty scan is a broken capture, not a clean run" >&2
    tail -40 "$RAW" >&2
    rm -f "$OUT"
    exit 1
fi

echo "bench-capture: $cells cell(s) -> $OUT"
