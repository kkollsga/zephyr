#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${ZEPHYR_PERF_DIR:-"$ROOT/.artifacts/perf/latest"}
RUNS=${ZEPHYR_PERF_RUNS:-5}
INTERACT=${ZEPHYR_PERF_INTERACT:-0}
SOAK_SECONDS=${ZEPHYR_PERF_SOAK_SECONDS:-0}
MIN_STEADY_SAMPLES=${ZEPHYR_PERF_MIN_STEADY_SAMPLES:-$RUNS}
MIN_INTERACTION_SAMPLES=${ZEPHYR_PERF_MIN_INTERACTION_SAMPLES:-1}
MIN_SOAK_SAMPLES=${ZEPHYR_PERF_MIN_SOAK_SAMPLES:-1}
MAX_WINDOW_VISIBLE_MS=${ZEPHYR_PERF_MAX_WINDOW_VISIBLE_MS:-0}
MAX_FIRST_SUBMIT_MS=${ZEPHYR_PERF_MAX_FIRST_SUBMIT_MS:-0}
MAX_STEADY_FRAME_P95_MS=${ZEPHYR_PERF_MAX_STEADY_FRAME_P95_MS:-0}
MAX_GIO_EVENT_TO_SUBMIT_P95_MS=${ZEPHYR_PERF_MAX_GIO_EVENT_TO_SUBMIT_P95_MS:-0}
MAX_RSS_MIB=${ZEPHYR_PERF_MAX_RSS_MIB:-0}
MAX_SOAK_GROWTH_MIB=${ZEPHYR_PERF_MAX_SOAK_GROWTH_MIB:-0}
# Maximum gates are disabled when set to zero. Sample minimums are always
# enforced for the scenarios they apply to.
FIXTURE=${1:-"$ROOT/testdata/gui/mouse_fixture.go"}
STATE="$OUT/state"
APP="$STATE/Zephyr Perf Test.app"
BIN="$APP/Contents/MacOS/zephyr-perf-test"
DRIVER="$STATE/zephyr-gui-driver"
HOME_DIR="$STATE/home"
RESULTS="$OUT/launch.tsv"

[[ "$RUNS" =~ ^[1-9][0-9]*$ ]] || { echo "ZEPHYR_PERF_RUNS must be a positive integer" >&2; exit 2; }
[[ "$INTERACT" == "0" || "$INTERACT" == "1" ]] || { echo "ZEPHYR_PERF_INTERACT must be 0 or 1" >&2; exit 2; }
[[ "$SOAK_SECONDS" =~ ^[0-9]+$ ]] || { echo "ZEPHYR_PERF_SOAK_SECONDS must be a non-negative integer" >&2; exit 2; }
[[ "$MIN_STEADY_SAMPLES" =~ ^[0-9]+$ ]] || { echo "ZEPHYR_PERF_MIN_STEADY_SAMPLES must be a non-negative integer" >&2; exit 2; }
[[ "$MIN_INTERACTION_SAMPLES" =~ ^[0-9]+$ ]] || { echo "ZEPHYR_PERF_MIN_INTERACTION_SAMPLES must be a non-negative integer" >&2; exit 2; }
[[ "$MIN_SOAK_SAMPLES" =~ ^[0-9]+$ ]] || { echo "ZEPHYR_PERF_MIN_SOAK_SAMPLES must be a non-negative integer" >&2; exit 2; }
[[ -f "$FIXTURE" ]] || { echo "fixture not found: $FIXTURE" >&2; exit 2; }

validate_maximum() {
    local name=$1 value=$2
    [[ "$value" =~ ^[0-9]+([.][0-9]+)?$ ]] || {
        echo "$name must be a non-negative number" >&2
        exit 2
    }
}
validate_maximum ZEPHYR_PERF_MAX_WINDOW_VISIBLE_MS "$MAX_WINDOW_VISIBLE_MS"
validate_maximum ZEPHYR_PERF_MAX_FIRST_SUBMIT_MS "$MAX_FIRST_SUBMIT_MS"
validate_maximum ZEPHYR_PERF_MAX_STEADY_FRAME_P95_MS "$MAX_STEADY_FRAME_P95_MS"
validate_maximum ZEPHYR_PERF_MAX_GIO_EVENT_TO_SUBMIT_P95_MS "$MAX_GIO_EVENT_TO_SUBMIT_P95_MS"
validate_maximum ZEPHYR_PERF_MAX_RSS_MIB "$MAX_RSS_MIB"
validate_maximum ZEPHYR_PERF_MAX_SOAK_GROWTH_MIB "$MAX_SOAK_GROWTH_MIB"

mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" "$HOME_DIR"
# Each invocation is an independent sample set. Never aggregate traces, logs,
# or soak data left by an earlier invocation with a different run count.
rm -f "$OUT"/frame-*.jsonl "$OUT"/app-*.log "$OUT"/soak-*.tsv \
    "$OUT"/*-values.txt "$OUT"/*-values.txt.sorted

go build -trimpath \
    -ldflags '-X main.version=perf-test -X main.commit=local -X main.date=local' \
    -o "$BIN" ./cmd/zephyr
cp "$ROOT/Info.plist" "$APP/Contents/Info.plist"
cp "$ROOT/assets/icon.icns" "$APP/Contents/Resources/icon.icns"
plutil -replace CFBundleExecutable -string zephyr-perf-test "$APP/Contents/Info.plist"
plutil -replace CFBundleIdentifier -string com.kristianweb.zephyr.perf-test "$APP/Contents/Info.plist"
codesign --force --sign - "$APP" >/dev/null
swiftc -O -framework ApplicationServices -framework CoreGraphics \
    -o "$DRIVER" "$ROOT/tools/gui-driver/main.swift"

interaction_required=0
if [[ "$INTERACT" == "1" ]] || ((SOAK_SECONDS > 0)); then
    interaction_required=1
    permissions=$($DRIVER permissions)
    if ! grep -Eq '"postEvents"[[:space:]]*:[[:space:]]*true' <<<"$permissions"; then
        echo "Accessibility permission is required for interactive performance runs." >&2
        echo "$permissions" >&2
        exit 1
    fi
fi

stop_pid() {
    local pid=${1:-}
    [[ -n "$pid" ]] || return 0
    kill "$pid" 2>/dev/null || true
    for _ in {1..30}; do
        kill -0 "$pid" 2>/dev/null || return 0
        sleep 0.1
    done
    kill -9 "$pid" 2>/dev/null || true
}
trap 'stop_pid "${pid:-}"' EXIT INT TERM

printf 'run\twindow_visible_ms\tgo_start_to_first_submit_us\tfirst_frame_cpu_us\trss_kib\n' >"$RESULTS"
: >"$OUT/summary.txt"
for ((run = 1; run <= RUNS; run++)); do
    rm -rf "$HOME_DIR"
    mkdir -p "$HOME_DIR"
    trace_file="$OUT/frame-$run.jsonl"
    start=$($DRIVER time)
    env HOME="$HOME_DIR" XDG_CONFIG_HOME="$HOME_DIR/.config" \
        ZEPHYR_PERF_TRACE="$trace_file" \
        nohup "$BIN" "$FIXTURE" >"$OUT/app-$run.log" 2>&1 </dev/null &
    pid=$!
    visible=""
    for _ in {1..300}; do
        window_info=$("$DRIVER" windows "$pid")
        if grep -q '"id"' <<<"$window_info"; then
            visible=$($DRIVER time)
            break
        fi
        kill -0 "$pid" 2>/dev/null || {
            echo "Zephyr exited during performance run $run" >&2
            exit 1
        }
        sleep 0.01
    done
    [[ -n "$visible" ]] || { echo "no visible window in run $run" >&2; exit 1; }
    sleep 0.3
    if [[ "$INTERACT" == "1" ]]; then
        "$DRIVER" click "$pid" 260 165 left
        "$DRIVER" type "$pid" 'perf'
        "$DRIVER" scroll-lines "$pid" 600 300 -3
        "$DRIVER" key "$pid" z cmd
        sleep 0.3
    fi
    if ((SOAK_SECONDS > 0)); then
        soak_file="$OUT/soak-$run.tsv"
        printf 'elapsed_s\trss_kib\n' >"$soak_file"
        soak_start=$($DRIVER time)
        soak_deadline=$((soak_start + SOAK_SECONDS * 1000000000))
        iteration=0
        while :; do
            now=$($DRIVER time)
            ((now < soak_deadline)) || break
            "$DRIVER" click "$pid" 300 180 left
            "$DRIVER" type "$pid" 'x'
            "$DRIVER" key "$pid" z cmd
            if ((iteration % 2 == 0)); then
                "$DRIVER" scroll-lines "$pid" 600 300 -3
            else
                "$DRIVER" scroll-lines "$pid" 600 300 3
            fi
            rss_now=$(ps -p "$pid" -o rss= | tr -d ' ')
            printf '%s\t%s\n' "$(((now - soak_start) / 1000000000))" "$rss_now" >>"$soak_file"
            iteration=$((iteration + 1))
            sleep 1
        done
        if ((iteration < MIN_SOAK_SAMPLES)); then
            echo "soak run $run produced $iteration samples; require at least $MIN_SOAK_SAMPLES" >&2
            exit 1
        fi
        awk -F '\t' '
            NR > 1 { n++; elapsed[n]=$1; rss[n]=$2; if (n == 1) first=$2; last=$2; if ($2 > max) max=$2 }
            END {
                if (!n) exit
                warmIndex=int(n/5)+1
                if (warmIndex > n) warmIndex=n
                warm=rss[warmIndex]; peakWarm=warm
                for (i=warmIndex; i<=n; i++) {
                    if (rss[i] > peakWarm) peakWarm=rss[i]
                    x=elapsed[i]; y=rss[i]/1024
                    sx+=x; sy+=y; sxx+=x*x; sxy+=x*y; k++
                }
                denominator=k*sxx-sx*sx
                slope=denominator ? ((k*sxy-sx*sy)/denominator)*3600 : 0
                printf "soak_samples=%d start_rss_mib=%.1f warmed_rss_mib=%.1f end_rss_mib=%.1f max_rss_mib=%.1f growth_mib=%.1f warmed_growth_mib=%.1f peak_after_warm_growth_mib=%.1f warmed_slope_mib_per_hour=%.3f\n", n, first/1024, warm/1024, last/1024, max/1024, (last-first)/1024, (last-warm)/1024, (peakWarm-warm)/1024, slope
            }' \
            "$soak_file" | tee -a "$OUT/summary.txt"
    fi
    rss=$(ps -p "$pid" -o rss= | tr -d ' ')
    elapsed_ms=$(((visible - start) / 1000000))
    stop_pid "$pid"
    pid=""

    first_submit_us=$(sed -nE '/"event":"frame"/!d; /"first":true/!d; s/.*"sinceStartUs":([0-9]+).*/\1/p' "$trace_file" | head -n 1)
    first_frame_us=$(sed -nE '/"event":"frame"/!d; /"first":true/!d; s/.*"frameUs":([0-9]+).*/\1/p' "$trace_file" | head -n 1)
    [[ -n "$first_submit_us" && -n "$first_frame_us" ]] || {
        echo "performance run $run did not record a first frame" >&2
        exit 1
    }
    printf '%s\t%s\t%s\t%s\t%s\n' "$run" "$elapsed_ms" "$first_submit_us" "$first_frame_us" "$rss" | tee -a "$RESULTS"
done

awk -F '\t' 'NR > 1 {
        visible += $2; submit += $3; firstcpu += $4; rss += $5
        if (NR == 2 || $2 < visibleMin) visibleMin=$2
        if ($2 > visibleMax) visibleMax=$2
        if ($3 > submitMax) submitMax=$3
        if ($4 > firstcpuMax) firstcpuMax=$4
        if ($5 > rssMax) rssMax=$5
    }
    END { n=NR-1; if (n > 0) printf "runs=%d mean_window_visible_ms=%.1f min_window_visible_ms=%d max_window_visible_ms=%d mean_go_start_to_first_submit_ms=%.3f max_go_start_to_first_submit_ms=%.3f mean_first_frame_cpu_ms=%.3f max_first_frame_cpu_ms=%.3f mean_rss_mib=%.1f max_rss_mib=%.1f\n", n, visible/n, visibleMin, visibleMax, submit/n/1000, submitMax/1000, firstcpu/n/1000, firstcpuMax/1000, rss/n/1024, rssMax/1024 }' \
    "$RESULTS" | tee -a "$OUT/summary.txt"

startup_values="$OUT/startup-frame-values.txt"
steady_values="$OUT/steady-frame-values.txt"
gio_values="$OUT/gio-event-to-submit-values.txt"
sed -nE '/"event":"frame"/!d; /"first":true/!d; s/.*"frameUs":([0-9]+).*/\1/p' "$OUT"/frame-*.jsonl >"$startup_values"
sed -nE '/"event":"frame"/!d; /"first":true/d; s/.*"frameUs":([0-9]+).*/\1/p' "$OUT"/frame-*.jsonl >"$steady_values"
sed -nE 's/.*"gioEventToSubmitUs":([0-9]+).*/\1/p' "$OUT"/frame-*.jsonl >"$gio_values"

summarize_latency() {
    local label=$1
    local values=$2
    local sorted="$values.sorted"
    sort -n "$values" >"$sorted"
    awk -v label="$label" '
        { value[NR]=$1; sum += $1 }
        END {
            if (!NR) { printf "%s_count=0\n", label; exit }
            p50=int((NR*50+99)/100); p95=int((NR*95+99)/100); p99=int((NR*99+99)/100)
            printf "%s_count=%d mean_ms=%.3f p50_ms=%.3f p95_ms=%.3f p99_ms=%.3f max_ms=%.3f\n", label, NR, sum/NR/1000, value[p50]/1000, value[p95]/1000, value[p99]/1000, value[NR]/1000
        }' "$sorted" | tee -a "$OUT/summary.txt"
}

summarize_latency startup_frames "$startup_values"
summarize_latency steady_frames "$steady_values"
summarize_latency gio_event_to_submit "$gio_values"

summary_value() {
    local key=$1
    awk -v key="$key" '{ for (i=1; i<=NF; i++) { split($i, part, "="); if (part[1] == key) { print part[2]; exit } } }' "$OUT/summary.txt"
}

require_minimum() {
    local label=$1 actual=$2 minimum=$3
    if ((actual < minimum)); then
        echo "$label produced $actual samples; require at least $minimum" >&2
        exit 1
    fi
}

check_maximum() {
    local label=$1 actual=$2 maximum=$3
    awk -v maximum="$maximum" 'BEGIN { exit maximum > 0 ? 0 : 1 }' || return 0
    if ! awk -v actual="$actual" -v maximum="$maximum" 'BEGIN { exit !(actual <= maximum) }'; then
        echo "$label regression: $actual exceeds maximum $maximum" >&2
        exit 1
    fi
}

startup_count=$(summary_value startup_frames_count)
steady_count=$(summary_value steady_frames_count)
gio_count=$(summary_value gio_event_to_submit_count)
require_minimum first_frames "$startup_count" "$RUNS"
require_minimum steady_frames "$steady_count" "$MIN_STEADY_SAMPLES"
if ((interaction_required)); then
    require_minimum gio_event_to_submit "$gio_count" "$MIN_INTERACTION_SAMPLES"
fi

check_maximum window_visible_ms "$(summary_value max_window_visible_ms)" "$MAX_WINDOW_VISIBLE_MS"
check_maximum go_start_to_first_submit_ms "$(summary_value max_go_start_to_first_submit_ms)" "$MAX_FIRST_SUBMIT_MS"

steady_p95=$(awk '/^steady_frames_count=/ { for (i=1; i<=NF; i++) if ($i ~ /^p95_ms=/) { split($i,a,"="); print a[2] } }' "$OUT/summary.txt")
gio_p95=$(awk '/^gio_event_to_submit_count=/ { for (i=1; i<=NF; i++) if ($i ~ /^p95_ms=/) { split($i,a,"="); print a[2] } }' "$OUT/summary.txt")
check_maximum steady_frame_p95_ms "${steady_p95:-0}" "$MAX_STEADY_FRAME_P95_MS"
if ((interaction_required)); then
    check_maximum gio_event_to_submit_p95_ms "${gio_p95:-0}" "$MAX_GIO_EVENT_TO_SUBMIT_P95_MS"
fi
launch_max_rss=$(awk '{ for (i=1; i<=NF; i++) if ($i ~ /^max_rss_mib=/) { split($i,a,"="); if (!seen || a[2] > max) max=a[2]; seen=1 } } END { if (seen) print max; else print 0 }' "$OUT/summary.txt")
check_maximum rss_mib "$launch_max_rss" "$MAX_RSS_MIB"

if ((SOAK_SECONDS > 0)) && [[ "$MAX_SOAK_GROWTH_MIB" != "0" ]]; then
    max_soak_growth=$(awk '{ for (i=1; i<=NF; i++) if ($i ~ /^peak_after_warm_growth_mib=/) { split($i,a,"="); if (!seen || a[2] > max) max=a[2]; seen=1 } } END { if (seen) print max; else print 0 }' "$OUT/summary.txt")
    check_maximum soak_growth_mib "$max_soak_growth" "$MAX_SOAK_GROWTH_MIB"
fi
