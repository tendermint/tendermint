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

## Fuzzing

[Fuzzing](https://en.wikipedia.org/wiki/Fuzzing) of various system inputs.

See `./fuzz/README.md` for more details.
