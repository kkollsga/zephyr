#!/usr/bin/env bash
# Cumulative perf-drift check: the newest per-release capture in bench/history/
# against the one roughly --releases-back releases earlier.
#
# Why an anchor and not just "this release vs the last one": a gate that
# recaptures its own baseline every release structurally cannot see slow drift.
# 10% per release passes a 20% per-release threshold forever (doctrine R11).
# Recapturing does NOT clear a failing verdict here — only recovering the
# performance does.
#
# Exit codes:  0 PASS (or a reported non-verdict)   1 FAIL   2 VOID
#
# Read THIS script's exit code. `make bench-anchor` is a convenience wrapper
# only: make turns any non-zero recipe status into 2, so FAIL and VOID are
# indistinguishable through it. The release flow invokes this path directly.
#
# VOID means the instrument, not the code, is what moved: a control cell moved
# beyond tolerance, or the two captures were taken on different hosts, arches
# or Go series. A VOID is not a pass and not a failure — it says this
# comparison carries no information.
#
# Usage:
#   scripts/check-bench-anchor.sh [--releases-back N] [--threshold PCT]
#       [--metric median|min|mean] [--min-overlap N] [--history-dir DIR]
#       [--current FILE] [--control-tolerance PCT] [--no-retry]
#   scripts/check-bench-anchor.sh --self-test
set -euo pipefail
export LC_ALL=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

RELEASES_BACK=3
# 20%. Two consecutive captures of an unchanged tree on the release machine,
# under normal load, put this instrument's noise floor at 7.5% (largest
# per-cell median delta) with the worst control run-to-run spread at 4.3%, so
# 20% sits 2.7x above the floor. A cumulative threshold has to clear the floor
# of the instrument feeding it, and this one is deliberately not a per-commit
# sensitivity: a real 5% regression on one cell is inside the floor and this
# check will never see it. Say that rather than implying otherwise.
THRESHOLD=20
METRIC=median
MIN_OVERLAP=10
HISTORY_DIR="$ROOT/bench/history"
CURRENT=""
CONTROL_TOLERANCE=""
RETRY=1
SELF_TEST=0
# A capture's median/min shape; a shift beyond this warns and never votes.
DISPERSION_TOLERANCE=3.0

while (($#)); do
    case $1 in
    --releases-back) RELEASES_BACK=$2; shift 2 ;;
    --threshold) THRESHOLD=$2; shift 2 ;;
    --metric) METRIC=$2; shift 2 ;;
    --min-overlap) MIN_OVERLAP=$2; shift 2 ;;
    --history-dir) HISTORY_DIR=$2; shift 2 ;;
    --current) CURRENT=$2; shift 2 ;;
    --control-tolerance) CONTROL_TOLERANCE=$2; shift 2 ;;
    --no-retry) RETRY=0; shift ;;
    --self-test) SELF_TEST=1; RETRY=0; shift ;;
    -h | --help) sed -n '2,29p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done
: "${CONTROL_TOLERANCE:=$THRESHOLD}"
[[ "$METRIC" =~ ^(median|min|mean)$ ]] || { echo "--metric must be median, min or mean" >&2; exit 2; }

meta() { sed -n "s/^# $2=//p" "$1" | head -1; }

# Version-ordered, oldest first. Non-numeric components (0.1.0-alpha) sort as
# zero, so a pre-release and its final sort together; that is close enough for
# picking an anchor and never affects the comparison itself.
list_history() {
    [[ -d $HISTORY_DIR ]] || return 0
    find "$HISTORY_DIR" -maxdepth 1 -name '*.tsv' -type f | awk -F/ '
    {
        v = $NF; sub(/\.tsv$/, "", v)
        n = split(v, p, /[.-]/); key = ""
        for (i = 1; i <= 4; i++) {
            x = (i <= n && p[i] ~ /^[0-9]+$/) ? p[i] : 0
            key = key sprintf("%08d.", x)
        }
        print key "\t" $0
    }' | sort | cut -f2
}

# The environment guard. Two captures from different machines or Go series are
# not comparable, and reading them as drift is exactly the false alarm R11
# names — so the verdict is VOID, never PASS and never FAIL.
check_conditions() {
    local cur=$1 anc=$2 key mismatch=0
    for key in host arch go_series; do
        local a b
        a=$(meta "$anc" "$key"); b=$(meta "$cur" "$key")
        if [[ -z $a || -z $b ]]; then
            echo "  VOID: capture is missing the '$key' metadata header"
            mismatch=1
        elif [[ $a != "$b" ]]; then
            echo "  VOID: $key differs — anchor '$a', current '$b'"
            mismatch=1
        fi
    done
    return $mismatch
}

compare() {
    local anc=$1 cur=$2
    awk -v metric="$METRIC" -v thr="$THRESHOLD" -v ctol="$CONTROL_TOLERANCE" \
        -v minov="$MIN_OVERLAP" -v disptol="$DISPERSION_TOLERANCE" '
    function col(m) { return m == "median" ? 3 : (m == "min" ? 4 : 5) }
    function geomean(sum, cnt) { return cnt ? exp(sum / cnt) : 0 }
    function sortnames(v, cnt,   i, j, t) {
        for (i = 2; i <= cnt; i++) {
            t = v[i]
            for (j = i - 1; j >= 1 && v[j] > t; j--) v[j + 1] = v[j]
            v[j + 1] = t
        }
    }
    /^#/ { next }
    FNR == NR { av[$1] = $(col(metric)); amed[$1] = $3; amin[$1] = $4; next }
    { cv[$1] = $(col(metric)); cmed[$1] = $3; cmin[$1] = $4 }
    END {
        for (name in cv) if (name in av) common[++n] = name
        if (n == 0) { print "  VOID: the two captures share no benchmark names"; exit 2 }
        sortnames(common, n)
        for (i = 1; i <= n; i++) {
            name = common[i]
            if (amin[name] > 0) { as += log(amed[name] / amin[name]); ac++ }
            if (cmin[name] > 0) { cs += log(cmed[name] / cmin[name]); cc++ }
        }
        ad = geomean(as, ac); cd = geomean(cs, cc)
        printf "  dispersion (geo-mean median/min over %d common cell(s)): anchor %.3f, current %.3f\n", n, ad, cd
        if (ad > 0 && cd > 0) {
            shift = (cd / ad - 1) * 100
            if (shift > disptol || shift < -disptol)
                printf "  WARNING: dispersion shifted %+.1f%% (> %.0f%%) — the two captures have different distribution shapes, so treat the deltas below as suspect. Warning only; it does not vote.\n", shift, disptol
        }
        worst = -1e9; void = 0
        for (i = 1; i <= n; i++) {
            name = common[i]
            d = (cv[name] / av[name] - 1) * 100
            delta[name] = d
            if (name ~ /^BenchmarkControl_/) {
                ad2 = (d < 0) ? -d : d
                if (ad2 > ctol) {
                    printf "  VOID: control %s moved %+.1f%% (tolerance %.1f%%) — the instrument moved, not the code\n", name, d, ctol
                    void = 1
                }
                continue
            }
            ncmp++
            if (d > worst) { worst = d; worstname = name }
            if (d > thr) regressed[++nreg] = name
        }
        if (void) {
            print "  VOID: a control cell measures no Zephyr code, so its move is the machine or the toolchain. Re-measure; if it reproduces exactly, the control premise has expired and that is a finding about this check."
            exit 2
        }
        if (ncmp == 0) {
            print "  the intersection is controls only — nothing to compare; reporting and passing"
            exit 0
        }
        if (n < minov) {
            printf "  only %d common benchmark(s) (< %d) — too few for a verdict; reporting and passing\n", n, minov
            exit 0
        }
        printf "  %-58s %12s %12s %8s\n", "cell", "anchor", "current", "delta"
        for (i = 1; i <= n; i++) {
            name = common[i]
            mark = (delta[name] > thr) ? "  <-- REGRESSED" : ""
            printf "  %-58s %12.4g %12.4g %+7.1f%%%s\n", name, av[name], cv[name], delta[name], mark
        }
        if (nreg) {
            printf "\nFAIL: %d benchmark(s) drifted more than +%.1f%% since the anchor:\n", nreg, thr
            for (i = 1; i <= nreg; i++) printf "  - %s: %+.1f%%\n", regressed[i], delta[regressed[i]]
            print "Drift accumulated across releases without any single release tripping its own gate. Recapturing does NOT clear this — only recovering the performance does."
            exit 1
        }
        printf "\nPASS: no cumulative drift over +%.1f%% across %d benchmark(s) (worst %s at %+.1f%%).\n", thr, n, worstname, worst
        exit 0
    }
    ' "$anc" "$cur"
}

# Retry captures land under .artifacts/, which has no purge tier of its own, so
# this script is their bound and their owner (R4). The newest RETRY_KEEP
# survive each write; the rest are the evidence of runs already reported on.
# Ordering is by the UTC timestamp in the name, not by the whole name: version
# prefixes sort 0.1.10 before 0.1.9 and would strand the newest file.
RETRY_KEEP=5
prune_retry_captures() {
    local dir=$1 keep=$2 f n=0
    [[ -d $dir ]] || return 0
    while IFS= read -r f; do
        n=$((n + 1))
        ((n > keep)) && rm -f "$f"
    done < <(find "$dir" -maxdepth 1 -name '*-retry-*.tsv' -type f |
        awk -F'-retry-' '{ print $NF "\t" $0 }' | sort -r | cut -f2)
    return 0
}

abspath() {
    local d b
    d=$(dirname "$1"); b=$(basename "$1")
    printf '%s/%s\n' "$(cd "$d" && pwd)" "$b"
}

# Bash 3.2 is what macOS ships, so the history list is a newline-separated
# string addressed with sed rather than an array with negative indices.
run_check() {
    local list n cur anchor oldest idx rc retry retry_dir capture_rc

    list=$(list_history)
    if [[ -n $CURRENT ]]; then
        cur=$(abspath "$CURRENT")
        list=$(printf '%s\n' "$list" | grep -vxF "$cur" || true)
    fi
    n=$(printf '%s\n' "$list" | grep -c . || true)

    if [[ -z $CURRENT ]]; then
        if ((n == 0)); then
            echo "bench anchor: no captures in $HISTORY_DIR — nothing to compare"
            echo "PASS (with note): the anchor has no history yet. The release that"
            echo "  first writes into bench/history/ cannot have a past to compare"
            echo "  against; that is not a failure."
            return 0
        fi
        cur=$(printf '%s\n' "$list" | sed -n "${n}p")
        n=$((n - 1))
        ((n > 0)) && list=$(printf '%s\n' "$list" | sed -n "1,${n}p") || list=""
    fi

    if ((n == 0)); then
        echo "bench anchor: $(basename "$cur") is the only capture — nothing to compare"
        echo "PASS (with note): fewer than 2 captures in history. Deliberately not a"
        echo "  VOID — a release must not fail on the absence of its own past."
        return 0
    fi

    idx=$((n - RELEASES_BACK))
    ((idx < 1)) && idx=1
    anchor=$(printf '%s\n' "$list" | sed -n "${idx}p")
    oldest=$(printf '%s\n' "$list" | sed -n "1p")

    echo "bench anchor: $(basename "$cur") vs $(basename "$anchor") — ${METRIC}, threshold +${THRESHOLD}%, ${RELEASES_BACK} release(s) back over ${n} in history (oldest available: $(basename "$oldest"))"
    if ! check_conditions "$cur" "$anchor"; then
        echo "VOID: the captures were taken under different conditions, so this comparison carries no information (R11). Not a pass."
        return 2
    fi

    rc=0
    compare "$anchor" "$cur" || rc=$?
    ((rc != 1)) && return $rc
    ((RETRY == 0)) && return 1

    # A regression verdict is retried exactly once, from a fresh capture: a
    # real regression reproduces on the immediate recapture and machine noise
    # does not. Both captures are kept as evidence, and the retry never lands
    # in bench/history/ — that tier is one file per released version, written
    # only by the release flow and never rewritten.
    retry_dir="$ROOT/.artifacts/bench"
    mkdir -p "$retry_dir"
    retry="$retry_dir/$(basename "$cur" .tsv)-retry-$(date -u +%Y%m%dT%H%M%SZ).tsv"
    echo
    echo "bench anchor: regression verdict — recapturing once before believing it."
    capture_rc=0
    "$ROOT/scripts/bench-capture.sh" "$retry" || capture_rc=$?
    prune_retry_captures "$retry_dir" "$RETRY_KEEP"
    if ((capture_rc != 0)); then
        echo "VOID: the retry capture failed to run, so the first verdict stands unconfirmed."
        return 2
    fi
    echo
    echo "bench anchor (retry): $(basename "$retry") vs $(basename "$anchor")"
    if ! check_conditions "$retry" "$anchor"; then
        echo "VOID: the retry ran under different conditions from the anchor."
        return 2
    fi
    rc=0
    compare "$anchor" "$retry" || rc=$?
    echo
    echo "Evidence kept: first capture $cur, retry $retry."
    ((rc == 0)) && echo "The regression did not reproduce on the immediate recapture — machine noise, not code."
    return $rc
}

# ---------------------------------------------------------------------------
# --self-test: every verdict this script can return, observed on a fixture.
# A gate nobody has watched fail is decoration (R1).
# ---------------------------------------------------------------------------
fixture() {
    local out=$1 host=$2 shift_pct=$3 control_pct=$4 cells=$5 i base
    {
        echo "# host=$host"
        echo "# arch=arm64"
        echo "# go=go1.26.0"
        echo "# go_series=go1.26"
        echo "# count=7"
        printf '#name\tn\tmedian_ns\tmin_ns\tmean_ns\tb_per_op\tallocs_per_op\n'
        for ((i = 1; i <= cells; i++)); do
            base=$(awk -v i="$i" -v p="$shift_pct" 'BEGIN { printf "%.6g", (100 * i) * (1 + p / 100) }')
            printf 'BenchmarkFixture_%02d-10\t7\t%s\t%s\t%s\t0\t0\n' "$i" "$base" \
                "$(awk -v b="$base" 'BEGIN { printf "%.6g", b * 0.98 }')" "$base"
        done
        for name in BenchmarkControl_Hash-10 BenchmarkControl_SortInts-10; do
            base=$(awk -v p="$control_pct" 'BEGIN { printf "%.6g", 50000 * (1 + p / 100) }')
            printf '%s\t7\t%s\t%s\t%s\t0\t0\n' "$name" "$base" \
                "$(awk -v b="$base" 'BEGIN { printf "%.6g", b * 0.98 }')" "$base"
        done
    } >"$out"
}

expect() {
    local want=$1 label=$2
    shift 2
    local rc=0 out
    out=$("$@" 2>&1) || rc=$?
    if ((rc == want)); then
        printf 'self-test PASS  %-42s exit %d\n' "$label" "$rc"
    else
        printf 'self-test FAIL  %-42s exit %d, wanted %d\n' "$label" "$rc" "$want"
        echo "$out" | sed 's/^/      /'
        SELF_TEST_RC=1
    fi
}

self_test() {
    SELF_TEST_RC=0
    local tmp
    tmp=$(mktemp -d "${TMPDIR:-/tmp}/zephyr-anchor.XXXXXX")
    trap 'rm -rf "$tmp"' RETURN
    local me="${BASH_SOURCE[0]}"

    mkdir -p "$tmp/single" "$tmp/clean" "$tmp/inflated" "$tmp/control" "$tmp/host" "$tmp/thin"
    fixture "$tmp/single/0.1.0.tsv" zephyr-host 0 0 12

    for d in clean inflated control host thin; do
        cp "$tmp/single/0.1.0.tsv" "$tmp/$d/0.1.0.tsv"
        cp "$tmp/single/0.1.0.tsv" "$tmp/$d/0.1.1.tsv"
    done
    fixture "$tmp/clean/0.1.2.tsv" zephyr-host 4 0 12       # inside the threshold
    fixture "$tmp/inflated/0.1.2.tsv" zephyr-host 45 0 12   # every cell +45%
    fixture "$tmp/control/0.1.2.tsv" zephyr-host 0 55 12    # controls only
    fixture "$tmp/host/0.1.2.tsv" other-host 0 0 12         # different machine
    # The shift is past the threshold on purpose: with it, only the min-overlap
    # branch can return 0. At shift 0 the fixture passed on its own merits and
    # the assertion below held with the branch deleted (observed).
    fixture "$tmp/thin/0.1.2.tsv" zephyr-host 45 0 4        # 4 cells + 2 controls

    echo "self-test: fixtures under $tmp"
    expect 0 "no history at all -> PASS with note" bash "$me" --history-dir "$tmp/empty" --no-retry
    expect 0 "one capture only -> PASS with note" bash "$me" --history-dir "$tmp/single" --no-retry
    expect 0 "unchanged-ish -> PASS" bash "$me" --history-dir "$tmp/clean" --no-retry
    expect 1 "hand-inflated anchor -> FAIL" bash "$me" --history-dir "$tmp/inflated" --no-retry
    expect 2 "perturbed control -> VOID" bash "$me" --history-dir "$tmp/control" --no-retry
    expect 2 "host mismatch -> VOID" bash "$me" --history-dir "$tmp/host" --no-retry
    expect 0 "overlap below --min-overlap -> report+pass" bash "$me" --history-dir "$tmp/thin" --no-retry

    # The release flow reads this script's exit code directly, and the two
    # not-pass verdicts have to be distinguishable through it. `make` turns any
    # non-zero recipe status into 2, so `make bench-anchor` cannot carry the
    # distinction; that is why the skill and CLAUDE.md name the script by path.
    local fail_rc=0 void_rc=0
    bash "$me" --history-dir "$tmp/inflated" --no-retry >/dev/null 2>&1 || fail_rc=$?
    bash "$me" --history-dir "$tmp/control" --no-retry >/dev/null 2>&1 || void_rc=$?
    if ((fail_rc == void_rc)); then
        echo "self-test FAIL  FAIL and VOID both exit $fail_rc — the two verdicts are indistinguishable"
        SELF_TEST_RC=1
    else
        printf 'self-test PASS  %-42s FAIL %d, VOID %d\n' "verdicts are distinguishable" "$fail_rc" "$void_rc"
    fi

    # The retry tier under .artifacts/ has a bound and this script owns it
    # (R4). Eight captures in, five newest out.
    local rdir="$tmp/retries" stamp
    mkdir -p "$rdir"
    for stamp in 01 02 03 04 05 06 07 08; do
        : >"$rdir/0.1.2-retry-20260101T0000${stamp}Z.tsv"
    done
    prune_retry_captures "$rdir" 5
    local left
    left=$(find "$rdir" -name '*-retry-*.tsv' -type f | grep -c . || true)
    if [[ $left != 5 ]] ||
        [[ ! -f "$rdir/0.1.2-retry-20260101T000008Z.tsv" ]] ||
        [[ -f "$rdir/0.1.2-retry-20260101T000001Z.tsv" ]]; then
        echo "self-test FAIL  retry captures not pruned to the newest 5 ($left left)"
        SELF_TEST_RC=1
    else
        printf 'self-test PASS  %-42s 8 -> 5\n' "retry captures pruned to the newest"
    fi

    # The fixture generator has to be able to produce a red, or every "PASS"
    # above is vacuous. Prove the inflated fixture really is inflated.
    local a c
    a=$(awk -F'\t' '$1 == "BenchmarkFixture_01-10" { print $3 }' "$tmp/inflated/0.1.1.tsv")
    c=$(awk -F'\t' '$1 == "BenchmarkFixture_01-10" { print $3 }' "$tmp/inflated/0.1.2.tsv")
    awk -v a="$a" -v c="$c" 'BEGIN { exit !(c > a * 1.4) }' || {
        echo "self-test FAIL  the inflated fixture is not inflated ($a -> $c)"
        SELF_TEST_RC=1
    }

    if ((SELF_TEST_RC == 0)); then
        echo "self-test: all verdicts observed (PASS, PASS-with-note, FAIL, VOID)."
    else
        echo "self-test: FAILED"
    fi
    return $SELF_TEST_RC
}

if ((SELF_TEST)); then
    self_test
    exit $?
fi

rc=0
run_check || rc=$?
exit $rc
