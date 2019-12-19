## v0.32.9

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:

- [rpc/lib][\#4248](https://github.com/tendermint/tendermint/issues/4248) RPC client basic authentication support (@greg-szabo)
- [p2p] \#4053 Add `unconditional_peer_ids` and `persistent_peers_max_dial_period` config variables (see ADR-050) (@dongsam)
- [tools][\#4227](https://github.com/tendermint/tendermint/pull/4227) Implement `tendermint debug kill` and
  `tendermint debug dump` commands for Tendermint node debugging functionality. See `--help` in both
  commands for further documentation and usage.
- [cli] \#4234 Add `--db_backend and --db_dir` flags (@princesinha19)
- [cli] \#4113 Add optional `--genesis_hash` flag to check genesis hash upon startup
- [config] \#3831 Add support for [RocksDB](https://rocksdb.org/) (@Stumble)
- [metrics] \#4263 Add
  - `consensus_validator_power`: track your validators power
  - `consensus_validator_last_signed_height`: track at which height the validator last signed
  - `consensus_validator_missed_blocks`: total amount of missed blocks for a validator
    as gauges in prometheus for validator specific metrics

### IMPROVEMENTS:

### BUG FIXES:

- [rpc/lib][\#4051](https://github.com/tendermint/tendermint/pull/4131) Fix RPC client, which was previously resolving https protocol to http (@yenkhoon)
- [cs] \#4069 Don't panic when block meta is not found in store (@gregzaitsev)
