build:
	@sh -c "'$(CURDIR)/build.sh'"

push:
	@sh -c "'$(CURDIR)/push.sh'"

build_develop:
	docker build -t "tendermint/tendermint:develop" -f Dockerfile.develop .

build_check:
	docker build --quiet --no-cache -t "tendermint/tendermint:check" -f Dockerfile.check .
	# BELOW - go version in Docker
	docker run tendermint/tendermint:check go version
	# BELOW - sha256sum of tendermint binary built in Docker
	docker run tendermint/tendermint:check sha256sum /go/bin/tendermint
	docker rmi -f tendermint/tendermint:check

push_develop:
	docker push "tendermint/tendermint:develop"

.PHONY: build build_develop push push_develop
