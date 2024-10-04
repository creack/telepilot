PROTO_SRCS = api/v1/api.proto
MTLS_CLIENTS = alice bob dave
GO_SRCS = $(shell find . -type f -name '*.go')

.DEFAULT_GOAL = help

include make/docker.mk
include make/protobuf.mk
include make/protobuf-lint.mk
include make/mtls.mk
include make/go-lint.mk

# Main target, generate the go protobuf files if needed and generate the mtls certs.
.PHONY: all
all: build mtls

# Latest as of 2024-10-03.
GO_IMG = golang:1.23.2-alpine@sha256:9dd2625a1ff2859b8d8b01d8f7822c0f528942fe56cfe7a1e7c38d3b8d72d679
GO_BIN = docker run --rm -u "$(shell id -u):$(shell id -g)" -v "${PWD}:/src" -w /src -v $(shell go env GOMODCACHE || echo "${PWD}/.build/gomodcache"):/gomodcache -e GOCACHE=/src/.build/gocache -e GOMODCACHE=/gomodcache -e GOOS=linux -e GOARCH=amd64 ${GO_IMG}

.PHONY: build
build: bin/telepilot bin/telepilotd
bin/%: cmd/% ${GO_SRCS} ${PROTO_GENS}
	mkdir -p $(dir $@)
	${GO_BIN} go build -ldflags '-w -s' -o $@ ./cmd/${*}

.PHONY: test
test: proto mtls
	${GO_BIN} go test -v ./...

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
		echo '    make all         Same as "make build mtls"'; \
		echo '    make build       Build the binaries for the client and server'; \
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
