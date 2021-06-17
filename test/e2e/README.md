# End-to-End Tests

Spins up and tests Tenderdash networks in Docker Compose based on a testnet manifest. To run the CI testnet:

```sh
make
./build/runner -f networks/ci.toml
```

This creates and runs a testnet named `ci` under `networks/ci/` (determined by the manifest filename).

## Testnet Manifests

Testnets are specified as TOML manifests. For an example see [`networks/ci.toml`](networks/ci.toml), and for documentation see [`pkg/manifest.go`](pkg/manifest.go).

## Random Testnet Generation

Random (but deterministic) combinations of testnets can be generated with `generator`:

```sh
./build/generator -d networks/generated/

# Split networks into 8 groups (by filename)
./build/generator -g 8 -d networks/generated/
```

Multiple testnets can be run with the `run-multiple.sh` script:

```sh
./run-multiple.sh networks/generated/gen-group3-*.toml
```

## Test Stages

The test runner has the following stages, which can also be executed explicitly by running `./build/runner -f <manifest> <stage>`:

* `setup`: generates configuration files.

* `start`: starts Docker containers.

* `load`: generates a transaction load against the testnet nodes.

* `perturb`: runs any requested perturbations (e.g. node restarts or network disconnects).

* `wait`: waits for a few blocks to be produced, and for all nodes to catch up to it.

* `test`: runs test cases in `tests/` against all nodes in a running testnet.

* `stop`: stops Docker containers.

* `cleanup`: removes configuration files and Docker containers/networks.

Auxiliary commands:

* `logs`: outputs all node logs.

* `tail`: tails (follows) node logs until cancelled.

## Tests

Test cases are written as normal Go tests in `tests/`. They use a `testNode()` helper which executes each test as a parallel subtest for each node in the network.

### Running Manual Tests

To run tests manually, set the `E2E_MANIFEST` environment variable to the path of the testnet manifest (e.g. `networks/ci.toml`) and run them as normal, e.g.:

```sh
./build/runner -f networks/ci.toml start
E2E_MANIFEST=networks/ci.toml go test -v ./tests/...
```

Optionally, `E2E_NODE` specifies the name of a single testnet node to test.

These environment variables can also be specified in `tests/e2e_test.go` to run tests from an editor or IDE:

```go
func init() {
	// This can be used to manually specify a testnet manifest and/or node to
	// run tests against. The testnet must have been started by the runner first.
	os.Setenv("E2E_MANIFEST", "networks/ci.toml")
	os.Setenv("E2E_NODE", "validator01")
}
```

### Debugging Failures

If a command or test fails, the runner simply exits with an error message and
non-zero status code. The testnet is left running with data in the testnet
directory, and can be inspected with e.g. `docker ps`, `docker logs`, or
`./build/runner -f <manifest> logs` or `tail`. To shut down and remove the
testnet, run `./build/runner -f <manifest> cleanup`.

If the standard `log_level` is not detailed enough (e.g. you want "debug" level
logging for certain modules), you can change it in the manifest file.

Each node exposes a [pprof](https://golang.org/pkg/runtime/pprof/) server. To
find out the local port, run `docker port <NODENAME> 6060 | awk -F: '{print
$2}'`. Then you may perform any queries supported by the pprof tool. Julia
Evans has a [great
post](https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/) on this
subject.

```bash
export PORT=$(docker port full01 6060 | awk -F: '{print $2}')

go tool pprof http://localhost:$PORT/debug/pprof/goroutine
go tool pprof http://localhost:$PORT/debug/pprof/heap
go tool pprof http://localhost:$PORT/debug/pprof/threadcreate
go tool pprof http://localhost:$PORT/debug/pprof/block
go tool pprof http://localhost:$PORT/debug/pprof/mutex
```

## Enabling IPv6

Docker does not enable IPv6 by default. To do so, enter the following in
`daemon.json` (or in the Docker for Mac UI under Preferences → Docker Engine):

```json
{
  "ipv6": true,
  "fixed-cidr-v6": "2001:db8:1::/64"
}
```

## Benchmarking testnets

It is also possible to run a simple benchmark on a testnet. This is done through the `benchmark` command. This manages the entire process: setting up the environment, starting the test net, waiting for a considerable amount of blocks to be used (currently 100), and then returning the following metrics from the sample of the blockchain:

- Average time to produce a block
- Standard deviation of producing a block
- Minimum and maximum time to produce a block
