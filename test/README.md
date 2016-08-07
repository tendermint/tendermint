# Tendermint Tests

The unit tests (ie. the `go test` s) can be run with `make test`.
The integration tests can be run wtih `make test_integrations`.

Running the integrations test will build a docker container with latest tendermint
and run the following tests in docker containers:

- go tests, with --race
- app tests
	- dummy app over socket
	- counter app over socket
	- counter app over grpc
- p2p tests
	- start a local dummy app testnet on a docker network (requires docker version 1.10+)
	- send a tx on each node and ensure the state root is updated on all of them

If on a `release-x.x.x` branch, we also run

- `go test` for all our dependency libs (test/test_libs.sh)
- network_testing - benchmark a mintnet based cloud deploy using netmon

# Coverage

TODO!



