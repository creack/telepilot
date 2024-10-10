PROTO_SRCS = api/v1/api.proto
MTLS_CLIENTS = alice bob dave
GO_SRCS = $(shell find . -type f -name '*.go')

.DEFAULT_GOAL = help
.DELETE_ON_ERROR:

include make/docker.mk
include make/protobuf.mk
include make/protobuf-lint.mk
include make/mtls.mk
include make/go-lint.mk
include make/images.mk

# Main target, generate the go protobuf files if needed and generate the mtls certs.
.PHONY: all
all: build mtls images

CGO_ENABLED = 0
# Latest as of 2024-10-03.
GO_DEBUG_IMG = golang:1.23.2@sha256:adee809c2d0009a4199a11a1b2618990b244c6515149fe609e2788ddf164bd10
# Latest as of 2024-10-03.
GO_IMG = golang:1.23.2-alpine@sha256:9dd2625a1ff2859b8d8b01d8f7822c0f528942fe56cfe7a1e7c38d3b8d72d679
GO_BIN = docker run -e CGO_ENABLED=${CGO_ENABLED} --rm -u "$(shell id -u):$(shell id -g)" -v "${PWD}:/src" -w /src -v $(shell go env GOMODCACHE || echo "${PWD}/.build/gomodcache"):/gomodcache -e GOCACHE=/src/.build/gocache -e GOMODCACHE=/gomodcache -e GOOS=linux -e GOARCH=amd64 ${GO_IMG}

GOTEST_BIN = docker run --cgroupns=host --privileged --rm -v "${PWD}:/src" -w /src -v $(shell go env GOMODCACHE || echo "${PWD}/.build/gomodcache"):/gomodcache -e GOCACHE=/src/.build/gocache -e GOMODCACHE=/gomodcache -e GOOS=linux -e GOARCH=amd64 ${GO_IMG}

.PHONY: build
build: bin/telepilot bin/telepilotd
bin/%: ${GO_SRCS} ${PROTO_GENS}
	mkdir -p $(dir $@)
	${GO_BIN} go build -ldflags '-w -s' -o $@ ./cmd/${*}

.PHONY: build/debug
build/debug: bin/telepilot-debug bin/telepilotd-debug
build/debug: GO_IMG := ${GO_DEBUG_IMG}
build/debug: CGO_ENABLED := 1
bin/%-debug: ${GO_SRCS} ${PROTO_GENS}
	mkdir -p $(dir $@)
	${GO_BIN} go build -tags debug -race -o $@ ./cmd/${*}

.PHONY: test
test: mtls ${PROTO_GENS}
	${GOTEST_BIN} go test -tags=debug -v ./...

# Run all linters.
.PHONY: lint

# Cleanup.
.PHONY: clean fclean
clean:
	rm -rf bin
fclean: clean

.PHONY: help
help:
	@( \
		echo 'Usage:'; \
		echo '  General:'; \
		echo '    make all         Same as "make build mtls images"'; \
		echo '    make build       Build the binaries for the client and server'; \
		echo '    make build/debug Build the binaries with race detector and symbols.'; \
		echo '    make images      Download default images from Docker Hub.'; \
		echo '    make test        Run the tests'; \
		echo '  File generation:'; \
		echo '    make proto       Generate Go protobuf files'; \
		echo '    make mtls        Generate CA, Server & Clients certs'; \
		echo '  Lint:'; \
		echo '    make lint        Run all linters'; \
		echo '    make lint/go     Run the Go linter'; \
		echo '    make lint/proto  Run the Protobuf linter'; \
		echo '  Misc:'; \
		echo '    make clean       Remove build artifacts / cache'; \
		echo '    make fclean      Same as clean but also removes generated files'; \
		echo '    make help        This message.'; \
	) >&2
