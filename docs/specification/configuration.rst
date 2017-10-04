Configuration
=============

TendermintCore can be configured via a TOML file in
``$TMHOME/config.toml``. Some of these parameters can be overridden by
command-line flags.

Config parameters
~~~~~~~~~~~~~~~~~

The main config parameters are defined
`here <https://github.com/tendermint/tendermint/blob/master/config/config.go>`__.

-  ``abci``: ABCI transport (socket \| grpc). *Default*: ``socket``
-  ``db_backend``: Database backend for the blockchain and
   TendermintCore state. ``leveldb`` or ``memdb``. *Default*:
   ``"leveldb"``
-  ``db_dir``: Database dir. *Default*: ``"$TMHOME/data"``
-  ``fast_sync``: Whether to sync faster from the block pool. *Default*:
   ``true``
-  ``genesis_file``: The location of the genesis file. *Default*:
   ``"$TMHOME/genesis.json"``
-  ``log_level``: *Default*: ``"state:info,*:error"``
-  ``moniker``: Name of this node. *Default*: ``"anonymous"``
-  ``priv_validator_file``: Validator private key file. *Default*:
   ``"$TMHOME/priv_validator.json"``
-  ``prof_laddr``: Profile listen address. *Default*: ``""``
-  ``proxy_app``: The ABCI app endpoint. *Default*:
   ``"tcp://127.0.0.1:46658"``

-  ``consensus.max_block_size_txs``: Maximum number of block txs.
   *Default*: ``10000``
-  ``consensus.create_empty_blocks``: Create empty blocks w/o txs.
   *Default*: ``true``
-  ``consensus.create_empty_blocks_interval``: Block creation interval, even if empty.
-  ``consensus.timeout_*``: Various consensus timeout parameters
-  ``consensus.wal_file``: Consensus state WAL. *Default*:
   ``"$TMHOME/data/cs.wal/wal"``
-  ``consensus.wal_light``: Whether to use light-mode for Consensus
   state WAL. *Default*: ``false``

-  ``mempool.*``: Various mempool parameters

-  ``p2p.addr_book_file``: Peer address book. *Default*:
   ``"$TMHOME/addrbook.json"``. **NOT USED**
-  ``p2p.laddr``: Node listen address. (0.0.0.0:0 means any interface,
   any port). *Default*: ``"0.0.0.0:46656"``
-  ``p2p.pex``: Enable Peer-Exchange (dev feature). *Default*: ``false``
-  ``p2p.seeds``: Comma delimited host:port seed nodes. *Default*:
   ``""``
-  ``p2p.skip_upnp``: Skip UPNP detection. *Default*: ``false``

-  ``rpc.grpc_laddr``: GRPC listen address (BroadcastTx only). Port
   required. *Default*: ``""``
-  ``rpc.laddr``: RPC listen address. Port required. *Default*:
   ``"0.0.0.0:46657"``
-  ``rpc.unsafe``: Enabled unsafe rpc methods. *Default*: ``true``
