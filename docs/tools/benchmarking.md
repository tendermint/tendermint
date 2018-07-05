# tm-bench

Tendermint blockchain benchmarking tool:

- https://github.com/tendermint/tools/tree/master/tm-bench

For example, the following:

    tm-bench -T 10 -r 1000 localhost:46657

will output:

    Stats             Avg        Stdev      Max
    Block latency     6.18ms     3.19ms     14ms
    Blocks/sec        0.828      0.378      1
    Txs/sec           963        493        1811

## Quick Start

[Install Tendermint](https://github.com/tendermint/tendermint#install)

then run:

    tendermint init
    tendermint node --proxy_app=kvstore

    tm-bench localhost:46657

with the last command being in a seperate window.

## Usage

    tm-bench [-c 1] [-T 10] [-r 1000] [endpoints]

    Examples:
            tm-bench localhost:46657
    Flags:
      -T int
            Exit after the specified amount of time in seconds (default 10)
      -c int
            Connections to keep open per endpoint (default 1)
      -r int
            Txs per second to send in a connection (default 1000)
      -v    Verbose output

## Development

    make get_vendor_deps
    make test
