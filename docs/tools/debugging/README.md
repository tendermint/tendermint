---
order: 1
parent:
  title: Debugging
  order: 1
---

# Debugging

## Tendermint debug kill

Tendermint comes with a `debug` sub-command that allows you to kill a live
Tendermint process while collecting useful information in a compressed archive.
The information includes the configuration used, consensus state, network
state, the node' status, the WAL, and even the stack trace of the process
before exit. These files can be useful to examine when debugging a faulty
Tendermint process.

```bash
tendermint debug kill <pid> </path/to/out.zip> --home=</path/to/app.d>
```

will write debug info into a compressed archive. The archive will contain the
following:

```sh
├── config.toml
├── consensus_state.json
├── net_info.json
├── stacktrace.out
├── status.json
└── wal
```

Under the hood, `debug kill` fetches info from `/status`, `/net_info`, and
`/dump_consensus_state` HTTP endpoints, and kills the process with `-6`, which
catches the go-routine dump.

## Tendermint debug dump

Also, the `debug dump` sub-command allows you to dump debugging data into
compressed archives at a regular interval. These archives contain the goroutine
and heap profiles in addition to the consensus state, network info, node
status, and even the WAL.

```bash
tendermint debug dump </path/to/out> --home=</path/to/app.d>
```

will perform similarly to `kill` except it only polls the node and
dumps debugging data every frequency seconds to a compressed archive under a
given destination directory. Each archive will contain:

```sh
├── consensus_state.json
├── goroutine.out
├── heap.out
├── net_info.json
├── status.json
└── wal
```

Note: goroutine.out and heap.out will only be written if a profile address is
provided and is operational. This command is blocking and will log any error.

## Tendermint Inspect

Tendermint includes an `inspect` command for querying Tendermint's state store and block
store over Tendermint RPC.

When the Tendermint consensus engine detects inconsistent state, it will crash the
entire Tendermint process. 
While in this inconsistent state, a node running Tendermint's consensus engine will not start up. 
The `inspect` command runs only a subset of Tendermint's RPC endpoints for querying the block store
and state store. 
`inspect` allows operators to query a read-only view of the stage.
`inspect` does not run the consensus engine at all and can therefore be used to debug
processes that have crashed due to inconsistent state. 


To start the `inspect` process, run
```bash
tendermint inspect
```

### RPC endpoints
The list of available RPC endpoints can be found by making a request to the RPC port.
For an `inspect` process running on `127.0.0.1:26657`, navigate your browser to 
`http://127.0.0.1:26657/` to retrieve the list of enabled RPC endpoints.

Additional information on the Tendermint RPC endpoints can be found in the [rpc documentation](https://docs.tendermint.com/master/rpc).
