PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
DIST := dist
BIN := orcha

.PHONY: build build-run build-debug build-all clean test

# `make build` builds the production IPC binary (consumed by the Python SDK).
build:
	go build -o $(DIST)/$(BIN) ./cmd/orcha

# `make build-run` builds the user-facing CLI driver (real API calls).
build-run:
	go build -o $(DIST)/$(BIN)-run ./cmd/orcha-run

# `make build-debug` builds the developer debug entry point.
build-debug:
	go build -o $(DIST)/$(BIN)-debug ./cmd/orcha-debug

# `make build-all` cross-compiles the production binary for every release
# target. The -run and -debug helpers are dev-host-only by design.
build-all:
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out=$(DIST)/$(BIN)-$$os-$$arch$$ext; \
		echo "==> $$out"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "-s -w" -o $$out ./cmd/orcha || exit 1; \
	done

test:
	go test ./...

clean:
	rm -rf $(DIST)
