# tm-bench

Tendermint blockchain benchmarking tool:

- [https://github.com/tendermint/tendermint/tree/master/tools/tm-bench](https://github.com/tendermint/tendermint/tree/master/tools/tm-bench)

For example, the following: `tm-bench -T 30 -r 10000 localhost:26657`

will output:

```
Stats          Avg       StdDev     Max      Total
Txs/sec        3981      1993       5000     119434
Blocks/sec     0.800     0.400      1        24
```

NOTE: **tm-bench only works with build-in `kvstore` ABCI application**. For it
to work with your application, you will need to modify `generateTx` function.
In the future, we plan to support scriptable transactions (see
[\#1938](https://github.com/tendermint/tendermint/issues/1938)).

## Quick Start

### Docker

```
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint init
docker run -it --rm -v "/tmp:/tendermint" -p "26657:26657" --name=tm tendermint/tendermint node --proxy_app=kvstore

docker run -it --rm --link=tm tendermint/bench tm:26657
```

### Using binaries

[Install Tendermint](https://github.com/tendermint/tendermint#install)

then run:

```
tendermint init
tendermint node --proxy_app=kvstore

tm-bench localhost:26657
```

with the last command being in a separate window.

## Usage

```
Tendermint blockchain benchmarking tool.

Usage:
        tm-bench [-c 1] [-T 10] [-r 1000] [-s 250] [endpoints] [-output-format <plain|json> [-broadcast-tx-method <async|sync|commit>]]

Examples:
        tm-bench localhost:26657
Flags:
  -T int
        Exit after the specified amount of time in seconds (default 10)
  -broadcast-tx-method string
        Broadcast method: async (no guarantees; fastest), sync (ensures tx is checked) or commit (ensures tx is checked and committed; slowest) (default "async")
  -c int
        Connections to keep open per endpoint (default 1)
  -output-format string
        Output format: plain or json (default "plain")
  -r int
        Txs per second to send in a connection (default 1000)
  -s int
        The size of a transaction in bytes, must be greater than or equal to 40. (default 250)
  -v    Verbose output
```

## How stats are collected

These stats are derived by having each connection send transactions at the
specified rate (or as close as it can get) for the specified time.
After the specified time, it iterates over all of the blocks that were created
in that time.
The average and stddev per second are computed based off of that, by
grouping the data by second.

To send transactions at the specified rate in each connection, we loop
through the number of transactions.
If its too slow, the loop stops at one second.
If its too fast, we wait until the one second mark ends.
The transactions per second stat is computed based off of what ends up in the
block.

Note that there will be edge effects on the number of transactions in the first
and last blocks.
This is because transactions may start sending midway through when tendermint
starts building the next block, so it only has half as much time to gather txs
that tm-bench sends.
Similarly the end of the duration will likely end mid-way through tendermint
trying to build the next block.

Each of the connections is handled via two separate goroutines.

## Development

```
make get_vendor_deps
make test
```
