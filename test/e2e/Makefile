docker:
	docker build --tag tendermint/e2e-node -f docker/Dockerfile ../..

ci: runner
	./build/runner -f networks/ci.toml

app:
	go build -o build/app ./app

runner:
	go build -o build/runner ./runner

.PHONY: app ci docker runner
