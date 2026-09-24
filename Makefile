BINARY  := tsk
BINDIR  := $(HOME)/.local/bin
PKG     := ./cmd/tsk

.PHONY: all test lint check build install clean

all: test build

test:
	go vet ./...
	go test -race -count=1 ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run

# One-shot local gate: build + lint + test.
# Deliberately does NOT depend on `build` (which installs to $(BINDIR)); it
# compiles with a plain `go build ./...` so `make check` never installs anything.
check: lint test
	go build ./...

build:
	go build -o $(BINDIR)/$(BINARY) $(PKG)
	@echo "✓ installed to $(BINDIR)/$(BINARY)"

install: build

clean:
	rm -f $(BINDIR)/$(BINARY)
