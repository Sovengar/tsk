BINARY  := tsk
BINDIR  := $(HOME)/.local/bin
PKG     := ./cmd/tsk

.PHONY: all test lint build install clean

all: test build

test:
	go vet ./...
	go test -count=1 ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run

build:
	go build -o $(BINDIR)/$(BINARY) $(PKG)
	@echo "✓ installed to $(BINDIR)/$(BINARY)"

install: build

clean:
	rm -f $(BINDIR)/$(BINARY)
