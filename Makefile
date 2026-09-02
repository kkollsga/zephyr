.PHONY: build build-windows app run test bench fuzz perf clean vet fmt lint all baseline install-test docs-test gui-test-build gui-test-launch gui-test-stop gui-test-permissions gui-test-smoke gui-test-regression gate check-dev-docs

# Bounds for check-dev-docs. DEV_DOCS_MAX_MB caps the gitignored working
# folder; MIN_FREE_MB is a floor on free space of the volume the repo sits on,
# because the accumulation this guards against ends in ENOSPC and a bound
# protecting free space has to key on free space, not on one folder's du.
DEV_DOCS_MAX_MB ?= 256
MIN_FREE_MB     ?= 5000

BINARY    = zephyr
APP       = Zephyr.app
VERSION  ?= $(shell test -f VERSION && printf 'v%s' "$$(cat VERSION)" || git describe --tags --always --dirty 2>/dev/null || echo dev)
APP_VERSION ?= $(shell printf '%s' "$(VERSION)" | sed -E 's/^v//; s/[-+].*//' | grep -E '^[0-9]+(\.[0-9]+){0,2}$$' || echo 0.0.0)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/zephyr

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
		go build -ldflags "$(LDFLAGS) -H windowsgui" -o $(BINARY).exe ./cmd/zephyr

app: build
	@-pkill -x zephyr 2>/dev/null; sleep 0.2
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources
	cp $(BINARY) $(APP)/Contents/MacOS/
	cp Info.plist $(APP)/Contents/
	@if command -v plutil >/dev/null 2>&1; then \
		plutil -replace CFBundleVersion -string "$(APP_VERSION)" $(APP)/Contents/Info.plist; \
		plutil -replace CFBundleShortVersionString -string "$(APP_VERSION)" $(APP)/Contents/Info.plist; \
	fi
	cp assets/icon.icns $(APP)/Contents/Resources/
	codesign --force --sign - $(APP)

run: build
	./$(BINARY) $(ARGS)

test:
	go test ./... -count=1

bench:
	go test ./internal/buffer/ -bench=. -benchmem
	go test ./internal/highlight/ -bench=. -benchmem
	go test ./internal/fuzzy/ -bench=. -benchmem

fuzz:
	go test ./internal/buffer -run '^$$' -fuzz=FuzzPieceTableEditModel -fuzztime=30s
	go test ./internal/git -run '^$$' -fuzz=FuzzParseUnifiedDiff -fuzztime=30s

perf:
	./scripts/perf-test.sh

install-test:
	./scripts/install-test.sh

docs-test:
	./scripts/docs-test.sh

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf $(APP)
	go clean

fmt:
	gofmt -w .

lint: vet
	@echo "All checks passed"

all: vet test build

baseline:
	./scripts/baseline.sh

gui-test-build:
	./scripts/gui-test.sh build

gui-test-launch:
	./scripts/gui-test.sh launch

gui-test-stop:
	./scripts/gui-test.sh stop

gui-test-permissions:
	./scripts/gui-test.sh permissions --request

gui-test-smoke:
	./scripts/gui-test.sh smoke

gui-test-regression:
	./scripts/gui-test.sh regression

## Pre-commit / pre-push gate: the cheap checks, in order, stopping at the
## first failure. Deliberately excludes the GUI harness (needs a logged-in
## macOS session with Accessibility + Screen Recording) — see the note it
## prints. Add a step here only when it has a record of catching a real
## failure; everything else is CI's job.
gate: check-dev-docs vet test
	@echo ""
	@echo "gate: PASSED — check-dev-docs, vet, test."
	@echo "NOT covered by this gate: the macOS GUI harness. If this change"
	@echo "touches the UI, run 'make gui-test-build && make gui-test-launch'"
	@echo "then 'make gui-test-smoke' (or gui-test-regression) and read the"
	@echo "result. If it cannot run here — no GUI session, or missing"
	@echo "Accessibility / Screen Recording permission — SAY SO. A harness"
	@echo "that did not run is not a pass."

## Bound the gitignored working folders (doctrine R4: every file accumulation
## has a bound and an owner). Fails on size or on a low free-space floor; only
## warns about entries past their tier's purge lifetime, because deciding what
## is reproducible is a human call. Never deletes anything.
check-dev-docs:
	@free=$$(df -m . | awk 'NR==2 {print $$4}'); \
	if [ "$${free:-0}" -lt $(MIN_FREE_MB) ]; then \
		echo "FAIL: $${free} MB free on this volume (floor $(MIN_FREE_MB) MB)"; \
		echo "  heaviest local dirs:"; \
		du -sm dev-docs .artifacts Zephyr.app 2>/dev/null | sort -rn | sed 's/^/    /'; \
		exit 1; \
	fi; \
	echo "free space: $${free} MB (floor $(MIN_FREE_MB) MB)"
	@if [ ! -d dev-docs ]; then echo "no dev-docs/ — nothing to bound"; else \
		mb=$$(du -sm dev-docs | cut -f1); \
		stale=$$( { find dev-docs/bench/out -mindepth 1 -maxdepth 1 -mtime +14; \
		            find dev-docs/temp      -mindepth 1 -maxdepth 1 -mtime +14; \
		            find dev-docs/bin       -mindepth 1 -maxdepth 1 -mtime +7;  \
		            find inbox/read         -mindepth 1 -maxdepth 1 -mtime +7;  \
		          } 2>/dev/null ); \
		if [ "$${mb:-0}" -ge $(DEV_DOCS_MAX_MB) ]; then \
			echo "FAIL: dev-docs/ is $${mb} MB (limit $(DEV_DOCS_MAX_MB) MB)"; \
			du -sm dev-docs/* dev-docs/bench/* 2>/dev/null | sort -rn | head -8 | sed 's/^/    /'; \
			[ -z "$$stale" ] || { echo "  past their documented lifetime:"; echo "$$stale" | sed 's/^/    /'; }; \
			echo "  -> reclaim, or move anything irreproducible to a durable tier"; \
			exit 1; \
		fi; \
		echo "dev-docs/: $${mb} MB (limit $(DEV_DOCS_MAX_MB) MB)"; \
		[ -z "$$stale" ] || { echo "WARN: past their documented lifetime:"; \
		                      echo "$$stale" | sed 's/^/    /'; }; \
	fi
	@art=$$(du -sm .artifacts 2>/dev/null | cut -f1); \
	[ -z "$$art" ] || echo ".artifacts/: $${art} MB (scripts/perf-test.sh + scripts/baseline.sh write here; no purge tier — reclaim with rm -rf .artifacts)"
