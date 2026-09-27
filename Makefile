BINARY  := tsk
PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
PKG     := ./cmd/tsk

.PHONY: all test lint check build install uninstall clean overdue mutate mutate-diff

all: test build

test:
	go vet ./...
	go test -race -count=1 ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run

# One-shot local gate: build + lint + test.
# `check` never installs anything: `build` compiles into the repo-local bin/
# directory and the trailing `go build ./...` is a full-tree compile gate. The
# latter is kept in addition to `build` because `build` only compiles the main
# package reachable from $(PKG), while `go build ./...` also compiles packages
# that are not reachable from the binary.
check: build lint test
	go build ./...

# Compile into the repo-local artifact bin/$(BINARY) (gitignored). Never installs.
build:
	go build -o bin/$(BINARY) $(PKG)
	@echo "✓ built bin/$(BINARY)"

# Copy the already-built artifact to its final location. Never compiles.
# Honors PREFIX (`make install PREFIX=/opt/foo`) and staged installs
# (`make install DESTDIR=/tmp/stage` writes only under DESTDIR).
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 bin/$(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "✓ installed to $(DESTDIR)$(BINDIR)/$(BINARY)"

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

mutate:
	go tool gremlins unleash --workers 4 --timeout-coefficient 3 --output report.json

# gremlins silently falls back to the whole module when the diff is empty (base == HEAD),
# so fail fast instead of running a full-module run that looks diff-scoped.
mutate-diff:
	@if git diff --name-only main...HEAD | grep -q '\.go$$'; then \
		go tool gremlins unleash --diff main --workers 4 --timeout-coefficient 3 --output report.json; \
	else \
		echo "no .go changes vs main - nothing to mutate"; \
	fi

# Remove the repo-local artifact; the installed binary is `uninstall`'s job.
clean:
	rm -rf bin/

# Imprime la ruta del helper de vencimientos (informativo).
overdue:
	@echo "internal/model/overdue.go"
