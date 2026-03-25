.PHONY: build build-windows app run test bench clean vet fmt lint all

BINARY    = zephyr
APP       = Zephyr.app
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
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
		plutil -replace CFBundleVersion -string "$(VERSION)" $(APP)/Contents/Info.plist; \
		plutil -replace CFBundleShortVersionString -string "$(VERSION)" $(APP)/Contents/Info.plist; \
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
