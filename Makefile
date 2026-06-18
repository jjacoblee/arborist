# Arborist developer tasks. Requires Go 1.23+ (matching go.mod).
# Run `make help` to list targets.

BINARY  := arb
CMD     := ./cmd/arb
BIN_DIR := bin
# Where `make install` links the binary. PREFIX/bin should be on your PATH;
# /usr/local/bin is by default on macOS. Override with: make install PREFIX=~/.local
PREFIX      ?= /usr/local
INSTALL_DIR := $(PREFIX)/bin
# Version is derived from git when available, else "dev". Override with:
#   make build VERSION=0.1.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: all help build install uninstall install-go run test fmt fmt-check vet check tidy clean

all: check build

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## Compile the binary into ./bin
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

install: build ## Build and link arb into $(PREFIX)/bin (on your PATH); uses sudo only if needed
	@dir="$(INSTALL_DIR)"; src="$(CURDIR)/$(BIN_DIR)/$(BINARY)"; \
	if mkdir -p "$$dir" 2>/dev/null && [ -w "$$dir" ]; then \
		ln -sf "$$src" "$$dir/$(BINARY)"; \
	else \
		echo "Linking into $$dir (requires sudo)"; \
		sudo mkdir -p "$$dir" && sudo ln -sf "$$src" "$$dir/$(BINARY)"; \
	fi; \
	echo "Installed $(BINARY) -> $$dir/$(BINARY)"; \
	if command -v $(BINARY) >/dev/null 2>&1; then \
		echo "Run: $(BINARY) --version"; \
	else \
		echo "Note: add $$dir to your PATH (it is the default on macOS)."; \
	fi

uninstall: ## Remove the arb link installed by `make install`
	@dir="$(INSTALL_DIR)"; \
	rm -f "$$dir/$(BINARY)" 2>/dev/null || sudo rm -f "$$dir/$(BINARY)"; \
	echo "Removed $$dir/$(BINARY)"

install-go: ## Install into the Go bin dir instead (GOBIN or GOPATH/bin)
	go install -ldflags "$(LDFLAGS)" $(CMD)
	@bindir="$$(go env GOBIN)"; [ -n "$$bindir" ] || bindir="$$(go env GOPATH)/bin"; \
	echo "Installed $(BINARY) to $$bindir/$(BINARY)"; \
	case ":$$PATH:" in \
	*":$$bindir:"*) echo "$$bindir is on your PATH. Run: $(BINARY) --version" ;; \
	*) echo "WARNING: $$bindir is not on your PATH; add it or use 'make install' instead." ;; \
	esac

run: build ## Build then run (pass args via ARGS="--help")
	$(BIN_DIR)/$(BINARY) $(ARGS)

test: ## Run all tests
	go test ./...

fmt: ## Format all Go source in place
	gofmt -w .

fmt-check: ## Fail if any Go file is not gofmt-clean
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt:"; echo "$$unformatted"; exit 1; \
	fi

vet: ## Run go vet
	go vet ./...

check: fmt-check vet test ## Run all checks (use before committing)

tidy: ## Tidy go.mod / go.sum
	go mod tidy

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
