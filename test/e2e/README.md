# End-to-End Tests

Spins up and tests Tendermint networks in Docker Compose based on a testnet manifest file. To run the CI testnet:

```sh
$ make docker
$ make runner
$ ./build/runner -f networks/ci.toml
```

This creates a testnet named `ci` under `networks/ci` (given by manifest filename), spins up Docker containers, and runs tests against them.

## Testnet Manifests

Testnets are specified as TOML manifests. For an example see [`networks/ci.toml`](networks/ci.toml), and for documentation see [`pkg/manifest.go`](pkg/manifest.go).

Testnets take their name from the basename of the manifest, and create a directory next to it. For example, the `networks/ci.toml` testnet will be named `ci` and generated in `networks/ci/`.

## Test Stages

The test runner has the following stages, which can also be executed explicitly by running `./build/runner -f <manifest> <stage>`:

* `setup`: generates configuration files.

* `start`: starts Docker containers.

* `perturb`: runs any requested perturbations (e.g. node restarts or network disconnects).

* `wait`: waits for a few blocks to be produced, and for all nodes to catch up to it.

* `test`: runs test cases in `tests/` against all nodes in a running testnet.

* `stop`: stops Docker containers.

* `cleanup`: removes configuration files and Docker containers/networks.

## Tests

Test cases are written as normal Go tests in `tests/`. They use a test runner `testNode()` which executes the test as a parallel subtest against each node in the network.

### Running Manual Tests

To run tests manually, the `E2E_MANIFEST` environment variable gives the path to the testnet manifest (e.g. `networks/ci.toml`). Optionally, `E2E_NODE` can specify the name of the testnet node to test. Tests can then be run as normal, e.g.:

```sh
$ ./build/runner -f networks/ci.toml start
$ E2E_MANIFEST=networks/ci.toml go test -v ./tests/...
```

These environment variables can also be specified in `tests/e2e_test.go` to run tests from an editor or IDE:

```go
func init() {
	// This can be used to manually specify a testnet manifest and/or node to
	// run tests against. The testnet must have been started by the runner first.
	os.Setenv("E2E_MANIFEST", "networks/ci.toml")
	os.Setenv("E2E_NODE", "validator01")
}
```

## Debugging Failures

If a command or test fails, the runner simply exits with an error message and non-zero status code. The testnet is left running, and can be inspected e.g. with `docker ps` and `docker logs` and in the testnet directory. To shut down and remove the testnet, run `./build/runner -f <manifest> cleanup`.

## Enabling IPv6

Docker does not enable IPv6 by default. To do so, enter the following in `daemon.json` (or in the Docker for Mac UI under Preferences → Docker Engine):

```json
{
  "ipv6": true,
  "fixed-cidr-v6": "2001:db8:1::/64"
}
```
