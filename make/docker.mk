# Build Docker images.
#   Dockerfile: make/Dockerfile.<name>
#   Cache file: .build/docker-<name>
#   Docker image: <name>
#   Context: Repository root
.build/docker-%: make/Dockerfile.%
	@mkdir -p $(dir $@)
	docker build -t ${*} -f $< .
	@touch $@

.PHONY: clean-docker
clean-docker:
	rm -rf .build
clean: clean-docker
