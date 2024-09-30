# Inputs:
# - PROTO_SRCS: List of proto files to compile.

# Expected generated files using the Go/GRPC plugins.
PROTO_GENS = ${PROTO_SRCS:%.proto=%.pb.go} ${PROTO_SRCS:%.proto=%_grpc.pb.go}

PROTOC_IMG = protobuf
# To skip docker and use system's protoc:
#     mkdir -p .build && touch .build/docker-protobuf-grpc && export PROTOC_BIN=protoc
#     make <target>
PROTOC_BIN = docker run --rm -it -u "$(shell id -u):$(shell id -g)" -v "${PWD}:/proto" -w /proto ${PROTOC_IMG}

.PHONY: proto
proto: ${PROTO_GENS}

# The generated files depend on the Docker image to be built.
${PROTO_GENS}: .build/docker-${PROTOC_IMG}

# The generated files depend on their proto definition.
%.pb.go %_grpc.pb.go: %.proto
	${PROTOC_BIN} --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative $<

# Full cleanup.
.PHONY: fclean-proto
fclean-proto:
	rm -f ${PROTO_GENS}

fclean: fclean-proto
