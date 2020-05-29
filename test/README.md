# Tendermint Tests

Unit tests have build tag `unit`, and can be run with `make test` (i.e. go test -tags unit).
Component integration tests have build tag `integration`, and can be run with
`make test_integration`. All tests, including Docker-based end-to-end tests, can be run with
`make test_all`.

Running `make all` will build a docker container with local version of tendermint and run the
following tests in docker containers:

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
