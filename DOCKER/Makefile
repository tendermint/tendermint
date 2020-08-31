build:
	@sh -c "'$(CURDIR)/build.sh'"

push:
	@sh -c "'$(CURDIR)/push.sh'"

build_testing:
	docker build --tag tendermint/testing -f ./Dockerfile.testing .

build_amazonlinux_buildimage:
	docker build -t "tendermint/tendermint:build_c-amazonlinux" -f Dockerfile.build_c-amazonlinux .

.PHONY: build push build_testing build_amazonlinux_buildimage
