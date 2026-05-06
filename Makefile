PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
DIST      := dist
BIN       := orcha
REPO      := ryfoo/orcha
PY_DIR    := python
NPM_DIR   := npm

# VERSION is read from the file at the repo root and stamped into both the Go
# binary (via -ldflags) and the Python wheel (via a sed pass over __init__.py
# inside the release target). Edit VERSION to bump.
VERSION := $(shell cat VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build build-run build-debug build-all test clean \
        release-binaries release-manifest release-python release-npm release release-clean

# ----------------------------------------------------------------------------
# Local builds
# ----------------------------------------------------------------------------

# `make build` builds the production IPC binary (consumed by the Python SDK).
build:
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN) ./cmd/orcha

# `make build-run` builds the user-facing CLI driver (real API calls).
build-run:
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN)-run ./cmd/orcha-run

# `make build-debug` builds the developer debug entry point.
build-debug:
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN)-debug ./cmd/orcha-debug

# `make build-all` cross-compiles the production binary for every release
# target. The -run and -debug helpers are dev-host-only by design.
build-all:
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out=$(DIST)/$(BIN)-$$os-$$arch$$ext; \
		echo "==> $$out"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o $$out ./cmd/orcha || exit 1; \
	done

test: build
	go test ./...
	@if command -v python3 >/dev/null 2>&1 && python3 -c 'import pytest' 2>/dev/null; then \
		python3 -m pytest $(PY_DIR)/tests/; \
	else \
		echo "python3/pytest not available — skipping Python tests"; \
	fi

clean:
	rm -rf $(DIST)

# ----------------------------------------------------------------------------
# Release artifacts
# ----------------------------------------------------------------------------

# `make release` produces every artifact needed for a GitHub Release + PyPI
# + npm upload: 5 platform binaries, a manifest.json (URL+sha256 per binary),
# a Python wheel + sdist, and a packed npm tarball. All artifacts land in
# dist/. It does NOT push, tag, or upload anything — see RELEASE.md for the
# next steps.
release: release-clean release-binaries release-manifest release-python release-npm
	@echo ""
	@echo "Release artifacts ready in $(DIST)/ for v$(VERSION):"
	@ls -la $(DIST) | sed 's/^/  /'
	@echo ""
	@echo "Next: follow RELEASE.md to tag, upload, and publish."

release-clean:
	rm -rf $(DIST)
	@mkdir -p $(DIST)

release-binaries: build-all

release-manifest: release-binaries
	@echo "==> $(DIST)/manifest.json"
	@go run ./tools/genmanifest $(DIST) $(VERSION) $(REPO) > $(DIST)/manifest.json

# The Python build sees __init__.py as the source of __version__ (pyproject
# uses dynamic = ["version"], attr = "orcha.__version__"). We rewrite that
# line to match VERSION so the wheel's metadata matches the Go binary's.
release-python:
	@echo "==> Python wheel + sdist (v$(VERSION))"
	@sed -i.bak 's/^__version__ = ".*"$$/__version__ = "$(VERSION)"/' $(PY_DIR)/orcha/__init__.py
	@rm -f $(PY_DIR)/orcha/__init__.py.bak
	@cd $(PY_DIR) && python3 -m build --outdir ../$(DIST) >/dev/null
	@ls $(DIST)/orcha_dev-*.whl $(DIST)/orcha_dev-*.tar.gz | sed 's/^/    /'

# The npm build stamps package.json's "version" to match VERSION (so the
# wrapper resolves the right release tag at runtime), then `npm pack`s a
# publishable tarball into dist/. Requires npm 7+.
release-npm:
	@echo "==> npm tarball (v$(VERSION))"
	@sed -i.bak 's/"version": *"[^"]*"/"version": "$(VERSION)"/' $(NPM_DIR)/package.json
	@rm -f $(NPM_DIR)/package.json.bak
	@cd $(NPM_DIR) && npm pack --pack-destination ../$(DIST) >/dev/null
	@ls $(DIST)/orcha-dev-*.tgz | sed 's/^/    /'
