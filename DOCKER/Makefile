build:
	@sh -c "'$(CURDIR)/build.sh'"

push:
	@sh -c "'$(CURDIR)/push.sh'"

build_develop:
	docker build -t "tendermint/tendermint:develop" -f Dockerfile.develop .

push_develop:
	docker push "tendermint/tendermint:develop"

.PHONY: build build_develop push push_develop
