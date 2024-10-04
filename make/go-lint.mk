# Latest as of 2024-09-30.
GOLANGCI_IMG = golangci/golangci-lint:v1.61.0-alpine@sha256:61e2d68adc792393fcb600340fe5c28059638d813869d5b4c9502392a2fb4c96
GOLANGCI_BIN = docker run --rm -u "$(shell id -u):$(shell id -g)" -v "${PWD}:/src" -w /src -v $(shell go env GOMODCACHE || echo "${PWD}/.build/gomodcache"):/gomodcache -e GOCACHE=/src/.build/gocache -e GOMODCACHE=/gomodcache -e GOLANGCI_LINT_CACHE=/src/.build/golangcicache ${GOLANGCI_IMG}

.PHONY: lint/go
lint/go:
	${GOLANGCI_BIN} golangci-lint run ./...
lint: lint/go
