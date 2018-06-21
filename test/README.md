# Tendermint Tests

The unit tests (ie. the `go test` s) can be run with `make test`.
The integration tests can be run with `make test_integrations`.

Running the integrations test will build a docker container with local version of tendermint
and run the following tests in docker containers:

- go tests, with --race
	- includes test coverage
- app tests
	- kvstore app over socket
	- counter app over socket
	- counter app over grpc
- persistence tests
	- crash tendermint at each of many predefined points, restart, and ensure it syncs properly with the app
- p2p tests
	- start a local kvstore app testnet on a docker network (requires docker version 1.10+)
	- send a tx on each node and ensure the state root is updated on all of them
	- crash and restart nodes one at a time and ensure they can sync back up (via fastsync)
	- crash and restart all nodes at once and ensure they can sync back up
