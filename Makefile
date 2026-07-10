.PHONY: build build-windows app run test bench fuzz perf clean vet fmt lint all baseline install-test docs-test gui-test-build gui-test-launch gui-test-stop gui-test-permissions gui-test-smoke gui-test-regression

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
