BINARY  := tsk
PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
PKG     := ./cmd/tsk
MUTATE_BASE ?= main

# Mutation gate scope. Only packages with real test coverage are gated. cmd/ is the
# entry point and has no logic of its own, so it stays out; everything else is in.
# scripts/mutate.sh reads this variable, so `make mutate` and CI gate the same set.
#
# internal/tui was excluded until 2026-10-02: it had 472 surviving mutants, a third
# of the code duplicated across five files with its own copy of the index
# arithmetic, and seven mutants that hung the suite. It came in at 47 survivors out
# of 937, which means the other 890 are either killed or documented in
# .mutation-allowlist. The work that got it there was mostly extraction -- the
# duplicated arithmetic became pure functions in layout.go, and the dead branches
# went away -- rather than tests alone.
MUTATE_EXCLUDE ?= cmd/

.PHONY: all test lint check build install uninstall clean overdue mutate mutate-diff coverage coverage-check

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
	@scripts/mutate.sh --run

mutate-diff:
	@scripts/mutate.sh --diff

COVER_PROFILE ?= coverage.out

coverage: ## Coverage profile of the whole suite
	@go test -count=1 -covermode=atomic -coverprofile=$(COVER_PROFILE) ./... > /dev/null
	@echo "profile: $(COVER_PROFILE)"

coverage-check: coverage ## Gate: this change's DIFF at 100%, and the total against scripts/coverage-floor
	@scripts/diff-coverage.sh "$(COVER_PROFILE)" "$(MUTATE_BASE)"

# Remove the repo-local artifact; the installed binary is `uninstall`'s job.
clean:
	rm -rf bin/

# Prints the path of the overdue helper (informational).
overdue:
	@echo "internal/model/overdue.go"
