VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GOFLAGS := -buildvcs=false
LDFLAGS := -X ztutor/internal/version.Version=$(VERSION) \
           -X ztutor/internal/version.Commit=$(COMMIT) \
           -X ztutor/internal/version.BuildDate=$(DATE)

.PHONY: build build-client build-server build-licensegen build-coursepack build-full docker docker-push run run-server clean reset dev dev-server tuitest test vet lint manifest verify

build: build-client build-server

build-client:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o ztutor ./cmd/ztutor/

build-server:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o ztutord ./cmd/ztutord/

build-licensegen:
	go build $(GOFLAGS) -o licensegen ./cmd/licensegen/

build-coursepack:
	go build $(GOFLAGS) -o coursepack ./cmd/coursepack/

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
	VERSION=dev go run ./cmd/ztutor/

dev-server:
	$(RUN_ENV) VERSION=dev go run ./cmd/ztutord/

tuitest:
	go run ./cmd/tuitest/

test:
	go test ./...

vet:
	go vet ./...

lint:
	staticcheck ./...

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
