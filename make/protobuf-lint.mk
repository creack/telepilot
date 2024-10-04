# Inputs:
# - PROTO_SRCS: List of proto files to lint.

# Latest as of 2024-10-03.
BUF_IMG = bufbuild/buf:1.44.0@sha256:7e06f3027a16f8f63c25db6a1b5544866b677ad16e3907c14563c4a61d408228

BUF_BIN = docker run --rm -u "$(shell id -u):$(shell id -g)" -v "${PWD}:/proto" -w /proto -e HOME=/tmp ${BUF_IMG}

.PHONY: lint/proto lint/proto/lint lint/proto/format
lint/proto: lint/proto/lint lint/proto/format
lint: lint/proto

lint/proto/lint:
	${BUF_BIN} lint
lint/proto/format:
	${BUF_BIN} format -d --exit-code

.PHONY: lint/proto/fix
lint/proto/fix:
	${BUF_BIN} format -w
lint/fix: lint/proto/fix
