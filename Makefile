BINARY  := tsk
BINDIR  := $(HOME)/.local/bin
PKG     := ./cmd/taskd

.PHONY: all test build install clean

all: test build

test:
	go vet ./...
	go test -count=1 ./...

build:
	go build -o $(BINDIR)/$(BINARY) $(PKG)
	@echo "✓ installed to $(BINDIR)/$(BINARY)"

install: build

clean:
	rm -f $(BINDIR)/$(BINARY)
