# Tendermint

## Overview

This is a quick start guide. If you have a vague idea about how Tendermint works
and want to get started right away, continue. Otherwise, [review the documentation](http://tendermint.readthedocs.io/en/master/)

## Install

### Quick Install

On a fresh Ubuntu 16.04 machine can be done with [this script](https://git.io/vNLfY), like so:

```
curl -L https://git.io/vxWlX | bash
source ~/.profile
```

WARNING: do not run the above on your local machine.

The script is also used to facilitate cluster deployment below.

### Manual Install

Requires:
- `go` minimum version 1.9
- `$GOPATH` environment variable must be set
- `$GOPATH/bin` must be on your `$PATH` (see https://github.com/tendermint/tendermint/wiki/Setting-GOPATH)

To install Tendermint, run:

```
go get github.com/tendermint/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint
make get_tools && make get_vendor_deps
make install
```

Note that `go get` may return an error but it can be ignored.

Confirm installation:

```
$ tendermint version
0.15.0-381fe19
```

## Initialization

Running:

```
tendermint init
```

will create the required files for a single, local node.

These files are found in `$HOME/.tendermint`:

```
$ ls $HOME/.tendermint

config.toml  data  genesis.json  priv_validator.json
```

For a single, local node, no further configuration is required.
Configuring a cluster is covered further below.

## Local Node

Start tendermint with a simple in-process application:

```
tendermint node --proxy_app=kvstore
```

and blocks will start to stream in:

```
I[01-06|01:45:15.592] Executed block                               module=state height=1 validTxs=0 invalidTxs=0
I[01-06|01:45:15.624] Committed state                              module=state height=1 txs=0 appHash=
```

Check the status with:

```
curl -s localhost:46657/status
```

### Sending Transactions

With the kvstore app running, we can send transactions:

```
curl -s 'localhost:46657/broadcast_tx_commit?tx="abcd"'
```

and check that it worked with:

```
curl -s 'localhost:46657/abci_query?data="abcd"'
```

We can send transactions with a key and value too:

```
curl -s 'localhost:46657/broadcast_tx_commit?tx="name=satoshi"'
```

and query the key:

```
curl -s 'localhost:46657/abci_query?data="name"'
```

where the value is returned in hex.

## Cluster of Nodes

First create four Ubuntu cloud machines. The following was tested on Digital Ocean Ubuntu 16.04 x64 (3GB/1CPU, 20GB SSD). We'll refer to their respective IP addresses below as IP1, IP2, IP3, IP4.

Then, `ssh` into each machine, and execute [this script](https://git.io/vNLfY):

```
curl -L https://git.io/vNLfY | bash
source ~/.profile
```

This will install `go` and other dependencies, get the Tendermint source code, then compile the `tendermint` binary.

Next, `cd` into `docs/examples`. Each command below should be run from each node, in sequence:

```
tendermint node --home ./node1 --proxy_app=kvstore --p2p.seeds IP1:46656,IP2:46656,IP3:46656,IP4:46656
tendermint node --home ./node2 --proxy_app=kvstore --p2p.seeds IP1:46656,IP2:46656,IP3:46656,IP4:46656
tendermint node --home ./node3 --proxy_app=kvstore --p2p.seeds IP1:46656,IP2:46656,IP3:46656,IP4:46656
tendermint node --home ./node4 --proxy_app=kvstore --p2p.seeds IP1:46656,IP2:46656,IP3:46656,IP4:46656
```

Note that after the third node is started, blocks will start to stream in because >2/3 of validators (defined in the `genesis.json`) have come online. Seeds can also be specified in the `config.toml`. See [this PR](https://github.com/tendermint/tendermint/pull/792) for more information about configuration options.

Transactions can then be sent as covered in the single, local node example above.
