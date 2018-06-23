# tm-bench

Tendermint blockchain benchmarking tool:

- https://github.com/tendermint/tools/tree/master/tm-bench

For example, the following:

    tm-bench -T 10 -r 1000 localhost:26657

will output:

    Stats             Avg        Stdev      Max
    Txs/sec           833        427        1326     
    Blocks/sec        0.900      0.300      1

These stats are derived by sending transactions at the specified rate for the
specified time. After the specified time, it iterates over all of the blocks
that were created in that time. The average and stddev per second are computed
based off of that, by grouping the data by second.

## Quick Start

[Install Tendermint](https://github.com/tendermint/tendermint#install)

then run:

    tendermint init
    tendermint node --proxy_app=kvstore

    tm-bench localhost:26657

with the last command being in a seperate window.

## Usage

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

## Development

    make get_vendor_deps
    make test
