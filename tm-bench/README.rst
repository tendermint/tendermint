Benchmarking
============

tm-bench
--------

Tendermint blockchain benchmarking tool: https://github.com/tendermint/tools/tree/master/tm-bench

For example, the following:

::

    tm-bench -T 10 -r 1000 localhost:46657

will output:

::

    Stats             Avg        Stdev      Max
    Block latency     6.18ms     3.19ms     14ms
    Blocks/sec        0.828      0.378      1
    Txs/sec           963        493        1811

Quick Start
^^^^^^^^^^^

Docker
~~~~~~

::

    docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint:0.12.1 init
    docker run -it --rm -v "/tmp:/tendermint" -p "46657:46657" --name=tm tendermint/tendermint:0.12.1

    docker run -it --rm --link=tm tendermint/bench tm:46657

Binaries
~~~~~~~~

If **Linux**, start with:

::

    curl -L https://s3-us-west-2.amazonaws.com/tendermint/0.12.1/tendermint_linux_amd64.zip && sudo unzip -d /usr/local/bin tendermint_linux_amd64.zip && sudo chmod +x tendermint

if  **Mac OS**, start with:

::

    curl -L https://s3-us-west-2.amazonaws.com/tendermint/0.12.1/tendermint_darwin_amd64.zip && sudo unzip -d /usr/local/bin tendermint_darwin_amd64.zip && sudo chmod +x tendermint

then run:

::

    tendermint init
    tendermint node --proxy_app=dummy

    tm-bench localhost:46657

with the last command being in a seperate window.

Usage
^^^^^

::

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

Development
^^^^^^^^^^^

::

    make get_vendor_deps
    make test
