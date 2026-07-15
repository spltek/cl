BINARY_NAME := cl
BIN_DIR     := bin
CMD_PATH    := ./cmd/cl

.PHONY: help build test test-verbose cover vet fmt fmt-check clean install run

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Compile the cl binary into bin/
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@# On Apple Silicon, the kernel validates a binary's code signature
	@# on every launch, including Go's automatic ad-hoc signature. That
	@# signature can end up invalid after later file operations (e.g. a
	@# plain `cp`), which makes the OS SIGKILL the binary on launch
	@# ("zsh: killed", exit code 137) even though the file itself is
	@# fine. Re-signing ad-hoc here keeps the signature consistent with
	@# whatever ends up on disk. No-op on non-macOS (no codesign).
	@if command -v codesign >/dev/null 2>&1; then \
		codesign --sign - --force $(BIN_DIR)/$(BINARY_NAME); \
	fi

test: ## Run the test suite
	go test ./...

test-verbose: ## Run the test suite with verbose output
	go test -v ./...

cover: ## Run the test suite with coverage report
	go test -cover ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go source files
	gofmt -w .

fmt-check: ## Fail if any Go source file is not gofmt-formatted
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) dist

install: build ## Install the binary into $GOPATH/bin (or GOBIN)
	go install $(CMD_PATH)

run: build ## Build then run the binary, forwarding extra args (make run ARGS="-add foo")
	./$(BIN_DIR)/$(BINARY_NAME) $(ARGS)
