# End-to-End Tests

Spins up and tests Tenderdash networks in Docker Compose based on a testnet manifest. To run the CI testnet:

```sh
make
./build/runner -f networks/ci.toml
```

This creates and runs a testnet named `ci` under `networks/ci/`.

## Conceptual Overview

End-to-end testnets are used to test Tendermint functionality as a user would use it, by spinning up a set of nodes with various configurations and making sure the nodes and network behave correctly. The background for the E2E test suite is outlined in [RFC-001](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-066-e2e-testing.md).

The end-to-end tests can be thought of in this manner:

1. Does a certain (valid!) testnet configuration result in a block-producing network where all nodes eventually reach the latest height?

2. If so, does each node in that network satisfy all invariants specified by the Go E2E tests?

The above should hold for any arbitrary, valid network configuration, and that configuration space  should be searched and tested by randomly generating testnets.

A testnet configuration is specified as a TOML testnet manifest (see below). The testnet runner uses the manifest to configure a set of Docker containers and start them in some order. The manifests can be written manually (to test specific configurations) or generated randomly by the testnet generator (to test a wide range of configuration permutations).

When running a testnet, the runner will first start the Docker nodes in some sequence, submit random transactions, and wait for the nodes to come online and the first blocks to be produced. This may involve e.g. waiting for nodes to block sync and/or state sync. If specified, it will then run any misbehaviors (e.g. double-signing) and perturbations (e.g. killing or disconnecting nodes). It then waits for the testnet to stabilize, with all nodes online and having reached the latest height.

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

In order to generate network configurations with settings for dash, you have to override a default preset with `dash`.

```sh
./build/generator -d networks/generated/ -p dash
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

### Full-node keys

Since Full nodes do not participate in the consensus process, then keeping validators' public keys also is not required.
By default, public keys are reset during network generation. However, if you need to keep its, then use this env
parameter `FULLNODE_PUBKEY_KEEP=true`.

For instance:

```sh
FULLNODE_PUBKEY_KEEP=true make runner/dashcore
```

### Speed up running e2e tests

Running the e2e tests using `make runner {network}` takes time because the
command builds docker image every time when you run it which is not
necessarily. Therefore, to speed up the launch of the e2e test you can manually
create an image once and use this image to run the tests with a new app version
for testing.
Also, you have to set `PRE_COMPILED_APP_PATH` with path to compiled `app`, by
default compiled files are put in `build` folder.

Look into `Makefile` to find all available commands for fast running, every
command should have prefix `runner/`.
The command compiles tenderdash using prebuild `tenderdash/e2e-node` docker
image.

For instance, this command runs e2e test for `dashcore` network using
precompiled app binary:

```bash
PRE_COMPILED_APP_PATH=/go/src/tenderdash/test/e2e/build/app
make runner/dashcore
```

### Debugging Failures

#### Logs

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

#### Delve debugger

You can run container binaries using the Delve debugger.
To enable Delve, set the `DEBUG` environment variable when setting up the runner:

```bash
DEBUG=1 ./build/runner -f networks/ci.toml setup
```

If you set DEBUG to `stop`, the app won't start automatically.
You'll need to connect to each app (each container) with your debugger
and start it manually.

NOTE: Right now, only the built-in app is supported(the one using
`entrypoint-builtin` script)

Containers expose DLV on ports starting from 40001 upwards.

Example configuration for Visual Studio Code `launch.json`:

```json
{
	"name": "E2E Docker 1",
	"type": "go",
	"request": "attach",
	"mode": "remote",
	"remotePath": "/src/tenderdash",
	"port": 40001,
	"host": "127.0.0.1"
}
```

For more details, see:

* [JetBrains configuration](https://blog.jetbrains.com/go/2020/05/06/debugging-a-go-application-inside-a-docker-container/)
* [Visual Studio Code configuration](https://medium.com/@kaperys/delve-into-docker-d6c92be2f823)

#### Core dumps

To analyze core dumps:

1. Examine [Dockerfile](docker/Dockerfile) to ensure `ENV TENDERMINT_BUILD_OPTIONS`  contains `nostrip` option AND `GOTRACEBACK` is set to `crash`, for example:

   ```docker
	ENV TENDERMINT_BUILD_OPTIONS badgerdb,boltdb,cleveldb,rocksdb,nostrip
	ENV GOTRACEBACK=crash
   ```

2. Build the container with `make`
3. On the **host** machine, set the location of core files:

   ```bash
   echo /core.%p | sudo tee /proc/sys/kernel/core_pattern
   ```

4. After the container stops due to panic, you can export its contents and run delve debugger:

	```bash
	CONTAINER=<container name>
	docker export -o ${CONTAINER}.tar ${CONTAINER}
	mkdir ${CONTAINER}
	cd ${CONTAINER}
	tar -xf ../${CONTAINER}.tar
	dlv core ./usr/bin/tenderdash ./core.*
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

## Benchmarking Testnets

It is also possible to run a simple benchmark on a testnet. This is done through the `benchmark` command. This manages the entire process: setting up the environment, starting the test net, waiting for a considerable amount of blocks to be used (currently 100), and then returning the following metrics from the sample of the blockchain:

- Average time to produce a block
- Standard deviation of producing a block
- Minimum and maximum time to produce a block

## Running Individual Nodes

The E2E test harness is designed to run several nodes of varying configurations within docker. It is also possible to run a single node in the case of running larger, geographically-dispersed testnets. To run a single node you can either run:

**Built-in**

```bash
make node
tendermint init validator
TMHOME=$HOME/.tendermint ./build/node ./node/built-in.toml
```

To make things simpler the e2e application can also be run in the tendermint binary
by running

```bash
tendermint start --proxy-app e2e
```

However this won't offer the same level of configurability of the application.

**Socket**

```bash
make node
tendermint init validator
tendermint start
./build/node ./node.socket.toml
```

Check `node/config.go` to see how the settings of the test application can be tweaked.
