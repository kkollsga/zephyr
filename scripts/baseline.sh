#!/usr/bin/env bash
set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${ZEPHYR_BASELINE_DIR:-"$ROOT/.artifacts/baseline/latest"}
mkdir -p "$OUT"
: >"$OUT/status.tsv"

# When go.mod selects a patch-newer auto toolchain, invoke that toolchain's Go
# binary directly. Mixing a host `go tool cover` with auto-downloaded package
# objects produces misleading compiler-version failures on macOS.
SELECTED_GOROOT=$(go env GOROOT)
if [[ -x "$SELECTED_GOROOT/bin/go" ]]; then
    export PATH="$SELECTED_GOROOT/bin:$PATH"
    export GOTOOLCHAIN=local
fi

run() {
    local name=$1
    shift
    echo "==> $name"
    local started=$SECONDS
    "$@" >"$OUT/$name.txt" 2>&1
    local rc=$?
    local elapsed=$((SECONDS - started))
    printf '%s\t%s\t%s\n' "$name" "$rc" "$elapsed" | tee -a "$OUT/status.tsv"
    return 0
}

cd "$ROOT"
{
    echo "date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "commit=$(git rev-parse HEAD)"
    echo "branch=$(git branch --show-current)"
    echo "go=$(go version)"
    echo "os=$(sw_vers -productVersion)"
    echo "arch=$(uname -m)"
} >"$OUT/environment.txt"

run modules go mod verify
run tidy go mod tidy -diff
run test go test ./... -count=1
run stress go test ./... -shuffle=on -count=10
run race go test -race ./... -count=1
run race-buffer go test -race ./internal/buffer \
    -run '^TestPieceTable_ConcurrentReads$' -count=10
run vet go vet ./...
run build go build -trimpath -o "$OUT/zephyr-release" ./cmd/zephyr
run install-test make install-test
run docs-test make docs-test
run coverage go test ./... -coverprofile="$OUT/coverage.out" -count=1
if [[ "${ZEPHYR_RUN_FUZZ:-0}" == "1" ]]; then
    run fuzz-buffer go test ./internal/buffer -run '^$' \
        -fuzz=FuzzPieceTableEditModel -fuzztime=30s
    run fuzz-diff go test ./internal/git -run '^$' \
        -fuzz=FuzzParseUnifiedDiff -fuzztime=30s
fi
run benchmarks go test \
    ./internal/buffer ./internal/fuzzy ./internal/git ./internal/highlight ./internal/navigator \
    -run '^$' -bench=. -benchmem -count=1
run staticcheck go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
if [[ "${ZEPHYR_RUN_SECURITY:-0}" == "1" ]]; then
    run vulnerabilities go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
fi

if [[ "$(uname -s)" == "Darwin" && -x "$ROOT/scripts/gui-test.sh" ]]; then
    permissions=$($ROOT/scripts/gui-test.sh permissions 2>/dev/null || true)
    if grep -q '"postEvents" : true' <<<"$permissions" && \
       grep -q '"screenCapture" : true' <<<"$permissions"; then
        run gui-regression make gui-test-regression
    else
        printf 'gui-regression\t77\t0\n' | tee -a "$OUT/status.tsv"
        printf '%s\n' "$permissions" >"$OUT/gui-regression.txt"
    fi
fi

echo
echo "Baseline results: $OUT"
column -t -s $'\t' "$OUT/status.tsv" 2>/dev/null || cat "$OUT/status.tsv"
