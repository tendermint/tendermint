NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

# Tendermint CLI

Some notes on using the command line `tendermint` tool. See [[Installation]] to get it set up.

The help menu is available at `tendermint --help` and the version number at `tendermint version`.

## Directory Root

The default directory for blockchain data is `~/.tendermint`. Override this with the `TMHOME` environment variable or `--home` flag.

## Initialize

`tendermint init` will initialize the `~/.tendermint` (or `$TMHOME`) directory with a new private key (`priv_validator.json`), and a `genesis.json` containing the associated public key.
This is all that's necessary to run a local testnet with one validator.

## Run a node

To run a tendermint node, use `tendermint node`.

Note by default it will try to connect to a tmsp app running on `0.0.0.0:46658`. 
Alternatively, specify an in-process tmsp app (eg. `tendermint node --proxy_app=dummy` to run the dummy app) or an different socket address (eg. `tendermint node --proxy_app=/var/run/tmsp.sock`).
To use grpc instead of the raw tmsp socket protocol, set `--tmsp=grpc`. 

[Fast-sync](Fast-sync) mode is enabled by default. It can be disabled with `--fast_sync=false`, and will turn off automatically once the node is synced up.

Specify seeds to connect to with a comma separated list, eg. `--seeds=seed1.mynodes.io:46656,seed2.mynodes.io:46656`

The logging level defaults to `notice`, but can be set to `error`, `warn`, `notice`, `info`, `debug` using `--log_level=<level>`.

## Reset

WARNING: UNSAFE. Only do this in development and only if you can afford to lose all blockchain data!

To reset a blockchain, remove the `~/.tendermint/data` directory and run `tendermint unsafe_reset_priv_validator`. This will reset the `priv_validator.json`, allowing you to sign blocks for heights you've already signed (doing this on a live network is considered a Byzantine fault!)