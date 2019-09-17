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

build_amazonlinux_buildimage:
	docker build -t "tendermint/tendermint:build_c-amazonlinux" -f Dockerfile.build_c-amazonlinux .

.PHONY: build build_develop push push_develop
