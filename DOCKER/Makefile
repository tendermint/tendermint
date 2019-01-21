build:
	@sh -c "'$(CURDIR)/build.sh'"

push:
	@sh -c "'$(CURDIR)/push.sh'"

build_develop:
	docker build -t "tendermint/tendermint:develop" -f Dockerfile.develop .

build_testing:
	docker build --tag tendermint/testing -f ./Dockerfile.testing .

push_develop:
	docker push "tendermint/tendermint:develop"

build_remote_val_harness:
	@IMAGE=remote_val_harness DOCKERFILE=Dockerfile.remote_val_harness sh -c "'$(CURDIR)/build.sh'"

push_remote_val_harness:
	@IMAGE=remote_val_harness sh -c "'$(CURDIR)/push.sh'"

.PHONY: build build_develop push push_develop
