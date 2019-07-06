# tm-bench

Tendermint blockchain benchmarking tool:

- [https://github.com/tendermint/tendermint/tree/master/tools/tm-bench](https://github.com/tendermint/tendermint/tree/master/tools/tm-bench)

For example, the following:

```
tm-bench -T 10 -r 1000 localhost:26657
```

will output:

```
Stats          Avg       StdDev     Max      Total
Txs/sec        818       532        1549     9000
Blocks/sec     0.818     0.386      1        9
```

## Quick Start

[Install Tendermint](../introduction/install.md)
This currently is setup to work on tendermint's develop branch. Please ensure
you are on that. (If not, update `tendermint` and `tmlibs` in gopkg.toml to use
the master branch.)

then run:

```
tendermint init
tendermint node --proxy_app=kvstore
```

```
tm-bench localhost:26657
```

with the last command being in a seperate window.

## Usage

```
tm-bench [-c 1] [-T 10] [-r 1000] [-s 250] [endpoints]

Examples:
        tm-bench localhost:26657
Flags:
  -T int
        Exit after the specified amount of time in seconds (default 10)
  -c int
        Connections to keep open per endpoint (default 1)
  -r int
        Txs per second to send in a connection (default 1000)
  -s int
        Size per tx in bytes
  -v    Verbose output
```

## How stats are collected

These stats are derived by having each connection send transactions at the
specified rate (or as close as it can get) for the specified time. After the
specified time, it iterates over all of the blocks that were created in that
time. The average and stddev per second are computed based off of that, by
grouping the data by second.

To send transactions at the specified rate in each connection, we loop
through the number of transactions. If its too slow, the loop stops at one second.
If its too fast, we wait until the one second mark ends. The transactions per
second stat is computed based off of what ends up in the block.

Each of the connections is handled via two separate goroutines.

## Development

```
make test
```
