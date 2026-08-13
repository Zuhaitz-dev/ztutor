VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GOFLAGS := -buildvcs=false
export GOFLAGS
LDFLAGS := -X ztutor/internal/version.Version=$(VERSION) \
           -X ztutor/internal/version.Commit=$(COMMIT) \
           -X ztutor/internal/version.BuildDate=$(DATE)
GOCACHE_DIR := $(CURDIR)/.cache/go-build
GO := GOCACHE=$(GOCACHE_DIR) go
GOFMT := gofmt
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')
STATICCHECK := $(or $(shell command -v staticcheck 2>/dev/null),$(shell go env GOPATH)/bin/staticcheck)
GOLANGCI_LINT := $(or $(shell command -v golangci-lint 2>/dev/null),$(shell go env GOPATH)/bin/golangci-lint)

# .exe suffix for Windows builds.
EXE := $(if $(filter windows,$(shell go env GOOS)),.exe)

# release helpers
SEMVER_RE := ^[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)?$$

define bump_validate
	@if [ -z "$(VER)" ]; then echo "Usage: make $(1) VER=x.y.z"; exit 1; fi
	@if ! echo "$(VER)" | grep -qE '$(SEMVER_RE)'; then echo "Version must be semver (e.g. 0.1.18, 0.2.0-rc1)"; exit 1; fi
	@if [ ! -f CHANGELOG.md ]; then echo "CHANGELOG.md not found"; exit 1; fi
	@if ! grep -q "^## v$(VER)" CHANGELOG.md; then echo "Section '## v$(VER)' not found in CHANGELOG.md"; exit 1; fi
endef

# preview: prints the tag that would be created without creating it.
# Usage: make bump-dry-run VER=0.1.18
bump-dry-run:
	$(call bump_validate,$@)
	@echo "Would create tag: v$(VER)"
	@awk -v ver="v$(VER)" '/^## v[0-9]/ && $$2 == ver { summary=$$0; sub(/^## v[^ ]* *-- */,"",summary); print "  message: " summary }' CHANGELOG.md

# create a versioned tag. CI builds and publishes assets.
# Usage: make bump VER=0.1.18
bump:
	$(call bump_validate,$@)
	@summary=$$(awk -v ver="v$(VER)" '/^## v[0-9]/ && $$2 == ver { s=$$0; sub(/^## v[^ ]* *-- */,"",s); print s }' CHANGELOG.md); \
	if [ -z "$$summary" ]; then echo "No summary found on '## v$(VER)' line in CHANGELOG.md"; exit 1; fi; \
	git tag -a "v$(VER)" -m "ztutor: $$summary"
	@echo "Tagged v$(VER). Push with: git push origin v$(VER)"

# extract release notes for a version from CHANGELOG.md.
# Usage: make release-notes VERSION=v0.1.18 > /tmp/body.md
release-notes:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release-notes VERSION=v0.1.18"; exit 1; fi
	@awk -v ver="$(VERSION)" ' \
		/^## v[0-9]/  { if (found) exit; if ($$2 == ver) found=1; next } \
		found        { sub(/^## v[^ ]* *-- */, "## "); print } \
	' CHANGELOG.md

.PHONY: build build-client build-server docker docker-push run run-server clean reset dev dev-server tuitest test test-race test-coverage check-coverage fuzz verify-mod vet fmt lint lint-fmt lint-vet lint-staticcheck lint-full manifest verify bump bump-dry-run release-notes manifest-verify

build: build-client build-server

$(GOCACHE_DIR):
	mkdir -p $@

build-client: | $(GOCACHE_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o ztutor$(EXE) ./cmd/ztutor/

build-server: | $(GOCACHE_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o ztutord$(EXE) ./cmd/ztutord/

IMAGE ?= ztutor

docker:
	docker build -f Dockerfile.prod \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg BUILD_DATE=$(DATE) \
	  -t $(IMAGE):$(VERSION) \
	  -t $(IMAGE):latest .

docker-push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

run: build-client
	./ztutor$(EXE)

run-server: build-server
	./ztutord$(EXE)

clean:
	rm -f ztutor ztutord ztutor.exe ztutord.exe ztutor.db ztutor_host_key

reset: clean
	rm -f $(HOME)/.local/share/ztutor/ztutor.db $(HOME)/.local/share/ztutor/ztutor_host_key
	rm -rf ./lessons
	@echo "Database, host key, and legacy lessons wiped. Next run starts fresh."
	@echo "Note: ./courses/ is preserved (course content is never removed)."

dev:
	VERSION=dev $(GO) run ./cmd/ztutor/

dev-server:
	VERSION=dev $(GO) run ./cmd/ztutord/

tuitest: | $(GOCACHE_DIR)
	$(GO) run ./cmd/tuitest/

test: manifest-verify | $(GOCACHE_DIR)
	$(GO) test ./...

test-race: | $(GOCACHE_DIR)
	$(GO) test -race -shuffle=on -count=1 ./...

# Produce coverage.out + a per-function summary.
test-coverage: | $(GOCACHE_DIR)
	$(GO) test -covermode=atomic -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

# Fail if any gated package's coverage drops below .coverage-thresholds.
check-coverage:
	./scripts/check-coverage.sh

# Run each Go fuzzer for FUZZTIME seconds (default 10).
FUZZTIME ?= 10
fuzz: | $(GOCACHE_DIR)
	@for pkg in $$(go list ./internal/... ./cmd/...); do \
		fuzzers="$$($(GO) test -run '^$$' "$$pkg" -list '^Fuzz' 2>/dev/null | grep '^Fuzz' || true)"; \
		for fz in $$fuzzers; do \
			echo "fuzzing $$pkg.$$fz for $(FUZZTIME)s"; \
			$(GO) test "$$pkg" -run '^$$' -fuzz "$$fz" -fuzztime $(FUZZTIME)s >/dev/null 2>&1 || true; \
		done; \
	done
	@echo "fuzz pass complete"

# Verify go.mod is tidy and the module sums verify.
verify-mod: | $(GOCACHE_DIR)
	$(GO) mod verify
	@diff="$$($(GO) mod tidy -diff 2>/dev/null)"; \
	if [ -n "$$diff" ]; then \
		echo "go.mod is not tidy. Run: go mod tidy"; \
		echo "$$diff"; \
		exit 1; \
	fi

manifest-verify:
	@backup=$$(mktemp -d); \
	trap 'rm -rf "$$backup"' EXIT; \
	for f in $$(find courses/ -name 'manifest.sha256' -type f 2>/dev/null); do \
		mkdir -p "$$backup/$$(dirname "$$f")"; \
		cp "$$f" "$$backup/$$f"; \
	done; \
	$(MAKE) -s manifest; \
	fail=0; \
	for f in $$(find courses/ -name 'manifest.sha256' -type f 2>/dev/null); do \
		if ! diff "$$f" "$$backup/$$f" > /dev/null 2>&1; then \
			cp "$$backup/$$f" "$$f"; \
			echo "ERROR: manifest out of date for $$f"; \
			echo "       Run 'make manifest' and commit the updated files."; \
			fail=1; \
		fi; \
	done; \
	exit $$fail

vet: | $(GOCACHE_DIR)
	$(GO) vet ./...

fmt:
	@out="$$( $(GOFMT) -w $(GOFILES) && $(GOFMT) -l $(GOFILES) )"; \
	if [ -n "$$out" ]; then \
		echo "gofmt left files unformatted:"; \
		echo "$$out"; \
		exit 1; \
	fi

lint: lint-fmt lint-vet

lint-fmt:
	@out="$$( $(GOFMT) -l $(GOFILES) )"; \
	if [ -n "$$out" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$out"; \
		exit 1; \
	fi

lint-vet: | $(GOCACHE_DIR)
	$(GO) vet ./...

lint-staticcheck:
	@if [ ! -x "$(STATICCHECK)" ]; then \
		echo "staticcheck not found. Install it with:"; \
		echo "  go install honnef.co/go/tools/cmd/staticcheck@latest"; \
		exit 1; \
	fi
	$(STATICCHECK) ./...

# Broad static analysis via golangci-lint (see .golangci.yml).
lint-full:
	@if [ ! -x "$(GOLANGCI_LINT)" ]; then \
		echo "golangci-lint not found. Install it with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	$(GOLANGCI_LINT) run ./...

MANIFEST_DIR ?= courses/

manifest:
	@for d in $(MANIFEST_DIR)*/; do \
		test -d "$$d" || continue; \
		for sec in "$$d"lessons/ "$$d"interviews/; do \
			test -d "$$sec" || continue; \
			(cd "$$sec" && find . -maxdepth 2 -name "lesson.md" | sort | xargs sha256sum > manifest.sha256 2>/dev/null); \
			echo "manifest written to $${sec}manifest.sha256"; \
		done; \
	done

verify:
	@fail=0; \
	for d in courses/*/; do \
		test -d "$$d" || continue; \
		for sec in "$$d"lessons/ "$$d"interviews/; do \
			test -f "$$sec"manifest.sha256 || continue; \
			(cd "$$sec" && sha256sum -c manifest.sha256 --quiet 2>/dev/null) || { \
				echo "$$sec FAILED"; fail=1; \
			}; \
		done; \
	done; \
	if [ $$fail -eq 0 ]; then echo "all course manifests verified OK"; else exit 1; fi
