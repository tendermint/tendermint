build:
	# TAG=0.8.0 TAG_NO_PATCH=0.8
	docker build -t "tendermint/tendermint" -t "tendermint/tendermint:$(TAG)" -t "tendermint/tendermint:$(TAG_NO_PATCH)" .

push:
	# TAG=0.8.0 TAG_NO_PATCH=0.8
	docker push "tendermint/tendermint:latest"
	docker push "tendermint/tendermint:$(TAG)"
	docker push "tendermint/tendermint:$(TAG_NO_PATCH)"

build_develop:
	docker build -t "tendermint/tendermint:develop" -f Dockerfile.develop .

push_develop:
	docker push "tendermint/tendermint:develop"

.PHONY: build build_develop push push_develop
