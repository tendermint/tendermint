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

## App

App tests app situations using abci-cli and tendermint.

## Fuzzing

[Fuzzing](https://en.wikipedia.org/wiki/Fuzzing) of various system inputs.

See `./fuzz/README.md` for more details.

## e2e - end to end 

End to end tests using docker-compose clusters

See `./e2e/README.md` for more details.

## Maverick

Maverick provides a mock to tests for byzantine fault tolerance in situations in which malicous agents are participating in the network.

See `./maverick/README.md` for more details.
