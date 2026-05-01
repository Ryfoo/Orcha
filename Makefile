PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
DIST := dist
BIN := orcha

.PHONY: build build-all clean test

build:
	go build -o $(DIST)/$(BIN) ./cmd/orcha

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
