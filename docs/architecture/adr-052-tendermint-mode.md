# ADR 052: Tendermint Mode

## Changelog

* 27-11-2019: Initial draft from ADR-051
* 13-01-2020: Separate ADR Tendermint Mode from ADR-051

## Context

- Fullnode mode: fullnode mode does not have the capability to become a validator. 
- Validator mode : this mode is exactly same as existing state machine behavior. sync without voting on consensus, and participate consensus when fully synced
- Seed mode : lightweight seed mode maintaining an address book, p2p like [TenderSeed](https://gitlab.com/polychainlabs/tenderseed)

## Decision

We would like to suggest a simple Tendermint mode abstraction. These modes will live under one binary, and when initializing a node the user will be able to specify which node they would like to create.

- Which reactor, component to include for each node
    - fullnode *(default)*
        - switch, transport
        - reactors
          - mempool
          - consensus
          - evidence
          - blockchain
          - p2p/pex
        - rpc (safe connections only)
        - *~~no privValidator(priv_validator_key.json, priv_validator_state.json)~~*
    - validator
        - switch, transport
        - reactors
          - mempool
          - consensus
          - evidence
          - blockchain
          - p2p/pex
        - rpc (safe connections only)
        - with privValidator(priv_validator_key.json, priv_validator_state.json)
    - seed
        - switch, transport
        - reactor
           - p2p/pex
- Configuration, cli command
    - We would like to suggest by introducing `mode` parameter in `config.toml` and cli
    - <span v-pre>`mode = "{{ .BaseConfig.Mode }}"`</span> in `config.toml`
    - `tendermint node --mode validator`  in cli
    - fullnode | validator | seed (default: "fullnode")
- RPC modification
    - `host:26657/status`
        - return empty `validator_info` when fullnode mode
    - no rpc server in seed mode
- Where to modify in codebase
    - Add  switch for `config.Mode` on `node/node.go:DefaultNewNode`
    - If `config.Mode==validator`, call default `NewNode` (current logic)
    - If `config.Mode==fullnode`, call `NewNode` with `nil` `privValidator` (do not load or generation)
        - Need to add exception routine for `nil` `privValidator` to related functions
    - If `config.Mode==seed`, call `NewSeedNode` (seed version of `node/node.go:NewNode`)
        - Need to add exception routine for `nil` `reactor`, `component` to related functions

## Status

Proposed

## Consequences

### Positive

- Node operators can choose mode when they run state machine according to the purpose of the node.
- Mode can prevent mistakes because users have to specify which mode they want to run via flag. (eg. If a user want to run a validator node, she/he should explicitly write down validator as mode)
- Different mode needs different reactors, resulting in efficient resource usage.

### Negative

- Users need to study how each mode operate and which capability it has.

### Neutral

## References

- Issue [#2237](https://github.com/tendermint/tendermint/issues/2237) : Tendermint "mode"
- [TenderSeed](https://gitlab.com/polychainlabs/tenderseed) : A lightweight Tendermint Seed Node.
