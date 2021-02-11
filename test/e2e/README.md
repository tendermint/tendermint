# End-to-End Tests

Spins up and tests Tendermint networks in Docker Compose based on a testnet manifest. To run the CI testnet:

```sh
make
./build/runner -f networks/ci.toml
```

This creates and runs a testnet named `ci` under `networks/ci/`.

## Conceptual Overview

End-to-end testnets are used to test Tendermint functionality as a user would use it, by spinning up a set of nodes with various configurations and making sure the nodes and network behave correctly. The background for the E2E test suite is outlined in [RFC-001](https://github.com/tendermint/tendermint/blob/master/docs/rfc/rfc-001-end-to-end-testing.md).

The end-to-end tests can be thought of in this manner:

1. Does a certain (valid!) testnet configuration result in a block-producing network where all nodes eventually reach the latest height?

2. If so, does each node in that network satisfy all invariants specified by the Go E2E tests?

The above should hold for any arbitrary, valid network configuration, and that configuration space  should be searched and tested by randomly generating testnets.

A testnet configuration is specified as a TOML testnet manifest (see below). The testnet runner uses the manifest to configure a set of Docker containers and start them in some order. The manifests can be written manually (to test specific configurations) or generated randomly by the testnet generator (to test a wide range of configuration permutations).

When running a testnet, the runner will first start the Docker nodes in some sequence, submit random transactions, and wait for the nodes to come online and the first blocks to be produced. This may involve e.g. waiting for nodes to fast sync and/or state sync. If specified, it will then run any misbehaviors (e.g. double-signing) and perturbations (e.g. killing or disconnecting nodes). It then waits for the testnet to stabilize, with all nodes online and having reached the latest height.

Once the testnet stabilizes, a set of Go end-to-end tests are run against the live testnet to verify network invariants (for example that blocks are identical across nodes). These use the RPC client to interact with the network, and should consider the entire network as a black box (i.e. it should not test any network or node internals, only externally visible behavior via RPC). The tests may use the `testNode()` helper to run parallel tests against each individual testnet node, and/or inspect the full blockchain history via `fetchBlockChain()`.

The tests must take into account the network and/or node configuration, and tolerate that the network is still live and producing blocks. For example, validator tests should only run against nodes that are actually validators, and take into account the node's block retention and/or state sync configuration to not query blocks that don't exist.

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

* `logs`: outputs all node logs.

* `tail`: tails (follows) node logs until canceled.

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
