.PHONY: build app run test bench clean vet

BINARY=zephyr
APP=Zephyr.app

build:
	go build -o $(BINARY) ./cmd/zephyr

app: build
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources
	cp $(BINARY) $(APP)/Contents/MacOS/
	cp Info.plist $(APP)/Contents/
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
	rm -f $(BINARY)
	rm -rf $(APP)
	go clean

fmt:
	gofmt -w .

lint: vet
	@echo "All checks passed"

all: vet test build
