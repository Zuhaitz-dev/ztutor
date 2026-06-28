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

.PHONY: build build-client build-server build-licensegen build-coursepack build-full docker docker-push run run-server clean reset dev dev-server tuitest test vet fmt lint lint-fmt lint-vet lint-staticcheck manifest verify

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

DEV_PUBKEY  := 0d11d19460424c2a9d8a14411acd3f0941197c736f38b8cca35913ee3e230de0
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

test: | $(GOCACHE_DIR)
	$(GO) test ./...

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

manifest:
	@for d in courses/*/; do \
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
