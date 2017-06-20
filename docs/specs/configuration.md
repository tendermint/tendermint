# Configuration

TendermintCore can be configured via a TOML file in `$TMHOME/config.toml`.
Some of these parameters can be overridden by command-line flags.

### Config parameters

The main config parameters are defined [here](https://github.com/tendermint/tendermint/blob/master/config/config.go).

* `abci`: ABCI transport (socket | grpc). _Default_: `socket`
* `db_backend`: Database backend for the blockchain and TendermintCore state.  `leveldb` or `memdb`.  _Default_: `"leveldb"`
* `db_dir`: Database dir.  _Default_: `"$TMHOME/data"`
* `fast_sync`: Whether to sync faster from the block pool.  _Default_: `true`
* `genesis_file`: The location of the genesis file.  _Default_: `"$TMHOME/genesis.json"`
* `log_level`: _Default_: `"state:info,*:error"`
* `moniker`: Name of this node.  _Default_: `"anonymous"`
* `priv_validator_file`: Validator private key file.  _Default_: `"$TMHOME/priv_validator.json"`
* `prof_laddr`: Profile listen address. _Default_: `""`
* `proxy_app`: The ABCI app endpoint.  _Default_: `"tcp://127.0.0.1:46658"`

* `consensus.max_block_size_txs`: Maximum number of block txs.  _Default_: `10000`
* `consensus.timeout_*`: Various consensus timeout parameters **TODO**
* `consensus.wal_file`: Consensus state WAL.  _Default_: `"$TMHOME/data/cswal"`
* `consensus.wal_light`: Whether to use light-mode for Consensus state WAL.  _Default_: `false`

* `mempool.*`: Various mempool parameters **TODO**

* `p2p.addr_book_file`: Peer address book.  _Default_: `"$TMHOME/addrbook.json"`.  **NOT USED**
* `p2p.laddr`: Node listen address. (0.0.0.0:0 means any interface, any port). _Default_: `"0.0.0.0:46656"`
* `p2p.pex`: Enable Peer-Exchange (dev feature). _Default_: `false`
* `p2p.seeds`: Comma delimited host:port seed nodes.  _Default_: `""`
* `p2p.skip_upnp`: Skip UPNP detection.  _Default_: `false`

* `rpc.grpc_laddr`: GRPC listen address (BroadcastTx only). Port required. _Default_: `""`
* `rpc.laddr`: RPC listen address. Port required. _Default_: `"0.0.0.0:46657"`
* `rpc.unsafe`: Enabled unsafe rpc methods. _Default_: `true`
