DOCKER_PROTO_BUILDER := docker run -v $(shell pwd):/workspace --workdir /workspace tendermintdev/docker-build-proto
HTTPS_GIT := https://github.com/tendermint/spec.git

###############################################################################
###                                Protobuf                                 ###
###############################################################################

proto-all: proto-lint proto-check-breaking
.PHONY: proto-all

proto-gen:
	@echo "Generating Protobuf files"
	@$(DOCKER_PROTO_BUILDER) buf generate --template=./buf.gen.yaml --config ./buf.yaml
.PHONY: proto-gen

proto-lint:
	@$(DOCKER_PROTO_BUILDER) buf lint --error-format=json --config ./buf.yaml
.PHONY: proto-lint

proto-format:
	@echo "Formatting Protobuf files"
	@$(DOCKER_PROTO_BUILDER) find . -name '*.proto' -path "./proto/*" -exec clang-format -i {} \;
.PHONY: proto-format

proto-check-breaking:
	@$(DOCKER_PROTO_BUILDER) buf breaking --against .git --config ./buf.yaml
.PHONY: proto-check-breaking

proto-check-breaking-ci:
	@$(DOCKER_PROTO_BUILDER) buf breaking --against $(HTTPS_GIT) --config ./buf.yaml
.PHONY: proto-check-breaking-ci
