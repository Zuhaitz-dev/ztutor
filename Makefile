VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GOFLAGS := -buildvcs=false
LDFLAGS := -X ztutor/internal/version.Version=$(VERSION) \
           -X ztutor/internal/version.Commit=$(COMMIT) \
           -X ztutor/internal/version.BuildDate=$(DATE)
GOCACHE_DIR := $(CURDIR)/.cache/go-build
GO := GOCACHE=$(GOCACHE_DIR) go
GOFMT := gofmt
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')
STATICCHECK := $(or $(shell command -v staticcheck 2>/dev/null),$(shell go env GOPATH)/bin/staticcheck)

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

.PHONY: build build-client build-server build-licensegen build-coursepack build-full docker docker-push run run-server clean reset dev dev-server tuitest test vet fmt lint lint-fmt lint-vet lint-staticcheck manifest verify bump bump-dry-run release-notes manifest-verify

build: build-client build-server

$(GOCACHE_DIR):
	mkdir -p $@

build-client: | $(GOCACHE_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o ztutor ./cmd/ztutor/

build-server: | $(GOCACHE_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o ztutord ./cmd/ztutord/

build-licensegen: | $(GOCACHE_DIR)
	$(GO) build $(GOFLAGS) -o licensegen ./cmd/licensegen/

build-coursepack: | $(GOCACHE_DIR)
	$(GO) build $(GOFLAGS) -o coursepack ./cmd/coursepack/

build-full: build-client build-server build-licensegen build-coursepack

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

DEV_PUBKEY  := aad959454bf169a828c90312863eafa064a4b507aa512ef19ca96b37a9d898ce
DEV_LICENSE := ./license_test.key

ifdef PREMIUM
RUN_ENV := ZTUTOR_LICENSE_PUBKEY=$(DEV_PUBKEY) ZTUTOR_LICENSE_FILE=$(DEV_LICENSE)
endif

run: build-client
	./ztutor

run-server: build-server
	$(RUN_ENV) ./ztutord

clean:
	rm -f ztutor ztutord licensegen coursepack ztutor.db ztutor_host_key

reset: clean
	rm -f $(HOME)/.local/share/ztutor/ztutor.db $(HOME)/.local/share/ztutor/ztutor_host_key
	rm -rf ./lessons
	@echo "Database, host key, and legacy lessons wiped. Next run starts fresh."
	@echo "Note: ./courses/ is preserved (course content is never removed)."

dev:
	VERSION=dev $(GO) run ./cmd/ztutor/

dev-server:
	$(RUN_ENV) VERSION=dev $(GO) run ./cmd/ztutord/

tuitest: | $(GOCACHE_DIR)
	$(GO) run ./cmd/tuitest/

test: manifest-verify | $(GOCACHE_DIR)
	$(GO) test ./...

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
