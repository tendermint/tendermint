# ADR 51: Double Signing Protection with Tendermint Mode

## Changelog
* 27-11-2019: Initial draft

## Context

There exists two main purposes for this ADR.

1. To introduce the simplest functional abstraction of Tendermint Mode
    - Fullnode mode : fullnode mode does not have any capability to participate on consensus
    - Validator mode : this mode is exactly same as existing state machine behavior. sync without voting on consensus, and participate consensus when fully synced
    - Seed mode : lightweight seed mode only for maintain an address book, p2p like [TenderSeed](https://gitlab.com/polychainlabs/tenderseed)
2. To provide a prevention method for double signing incidents mistakenly executed by validators
    - Validators often mistakenly run duplicated validators to cause double-signing incident
    - This proposed feature is to reduce the risk of mistaken double-signing incident by checking recent N blocks before voting begins
    - When we think of such serious impact on double-signing incident, it is very reasonable to have multiple protection algorithm built in node daemon

## Decision

We would like to suggest a simple Tendermint mode abstraction with a double signing prevention method.

### 1 ) Tendermint Mode

- Which reactor, component to include for each node
    - fullnode *(default)*
        - switch, transport
        - all reactors
        - mempool, state
        - rpc
        - *~~no privValidator(priv_validator_key.json, priv_validator_state.json)~~*
    - validator
        - switch, transport
        - all reactors
        - mempool, state
        - rpc
        - with privValidator(priv_validator_key.json, priv_validator_state.json)
    - seed
        - switch, transport
        - only pexReactor
- Configuration, cli command
    - We would like to suggest by introducing `mode` parameter in `config.toml` and cli
    - `mode = "{{ .BaseConfig.Mode }}"` in `config.toml`
    - `tendermint node --mode validator`  in cli
    - fullnode | validator | seed (default: "fullnode")
- RPC modification
    - `host:26657/status`
        - return empty `validator_info` when fullnode mode
        - add return `mode` of this node
    - no rpc server in seed mode
- Where to modify in codebase
    - Add  switch for `config.Mode` on `node/node.go:DefaultNewNode`
    - If `config.Mode==validator`, call default `NewNode` (current logic)
    - If `config.Mode==fullnode`, call `NewNode` with `nil` `privValidator` (do not load or generation)
        - Need to add exception routine for `nil` `privValidator` to related functions
    - If `config.Mode==seed`, call `NewSeedNode` (seed version of `node/node.go:NewNode`)
        - Need to add exception routine for `nil` `reactor`, `component` to related functions

### 2 ) Double signing prevention method

- Methodology : query recent consensus results to find out whether node's consensus key is used on consensus recently or not
- When to check
    - When the state machine starts `ConsensusReactor` after fully synced
    - When the node is validator ( with privValidator )
    - When `cs.config.DoubleSignCheckHeight > 0`
- How to check
    1. When a validator is transformed from syncing status to fully synced status, the state machine check recent N blocks to find out whether there exists consensus votes using the validator's consensus key
    2. If there exists votes from the validator's consensus key, exit state machine program
- Configuration
    - We would like to suggest by introducing `double_sign_check_height` parameter in `config.toml` and cli, how many blocks state machine looks back to check votes
    - `double_sign_check_height = {{ .Consensus.DoubleSignCheckHeight }}` in `config.toml`
    - `tendermint node --double_sign_check_height` in cli
    - State machine ignore checking procedure when `vote-check-height == 0`

## Status

Proposed

## Consequences

### Positive

1. Tendermint Mode
    - Node operators can choose mode when they run state machine according to the purpose of the node.
    - Mode can prevent mistakes because users have to specify which mode they want to run via flag. (eg. If a user want to run a validator node, she/he should explicitly write down validator as mode)
    - Different mode needs different reactors, resulting in efficient resource usage.
2. Double signing prevention method
    - Validators can avoid double signing incident by mistakes. (eg. If another validator node is voting on consensus, starting new validator node with same consensus key will cause panic stop of the state machine because consensus votes with the consensus key are found in recent blocks)
    - We expect this method will prevent majority of double signing incident by mistakes.

### Negative

1. Tendermint Mode
    - Users need to study how each mode operate and which capability it has.
2. Double signing prevention method
    - When the prevention method is on, restarting a validator node will panic because the node itself voted on consensus with the same consensus key. So, validators should stop the state machine, wait for some blocks, and then restart the state machine to avoid panic stop.

### Neutral

## References

- Issue [#4059](https://github.com/tendermint/tendermint/issues/4059) : double-signing protection
- Issue [#2237](https://github.com/tendermint/tendermint/issues/2237) : Tendermint "mode"
- [TenderSeed](https://gitlab.com/polychainlabs/tenderseed) : A lightweight Tendermint Seed Node.