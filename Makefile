.PHONY: build build-static test test-integration lint clean docker-build

BINARY    := minio-manager
CMD_PATH  := ./cmd/$(BINARY)

VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE      := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY) $(CMD_PATH)

# Fully static, portable linux/amd64 build with no glibc/NSS dependency.
# CGO_ENABLED=0 + netgo/osusergo force pure-Go networking and user lookups, so
# the binary runs on any x86_64 Linux (incl. scratch/alpine/musl) regardless of
# the host's libc. -extldflags '-static' documents intent (no-op without cgo).
build-static:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-trimpath \
		-tags 'netgo osusergo' \
		-ldflags="$(LDFLAGS) -extldflags '-static'" \
		-o bin/$(BINARY)-linux-amd64 $(CMD_PATH)

test:
	go test -race -count=1 ./...

test-integration:
	go test -tags=integration -race -count=1 -timeout=120s ./tests/integration/...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t $(BINARY):$(VERSION) .
