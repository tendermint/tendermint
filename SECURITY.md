# Security

As part of our [Coordinated Vulnerability Disclosure
Policy](https://tendermint.com/security), we operate a [bug
bounty](https://hackerone.com/tendermint).
See the policy for more details on submissions and rewards.

Here is a list of examples of the kinds of bugs we're most interested in:

## Specification

- Conceptual flaws
- Ambiguities, inconsistencies, or incorrect statements
- Mis-match between specification and implementation of any component

## Consensus

Assuming less than 1/3 of the voting power is Byzantine (malicious):

- Validation of blockchain data structures, including blocks, block parts,
  votes, and so on
- Execution of blocks
- Validator set changes
- Proposer round robin
- Two nodes committing conflicting blocks for the same height (safety failure)
- A correct node signing conflicting votes
- A node halting (liveness failure)
- Syncing new and old nodes

## Networking

- Authenticated encryption (MITM, information leakage)
- Eclipse attacks
- Sybil attacks
- Long-range attacks
- Denial-of-Service

## RPC

- Write-access to anything besides sending transactions
- Denial-of-Service
- Leakage of secrets

## Denial-of-Service

Attacks may come through the P2P network or the RPC:

- Amplification attacks
- Resource abuse
- Deadlocks and race conditions
- Panics and unhandled errors

## Libraries

- Serialization (Amino)
- Reading/Writing files and databases
- Logging and monitoring

## Cryptography

- Elliptic curves for validator signatures
- Hash algorithms and Merkle trees for block validation
- Authenticated encryption for P2P connections

## Light Client

- Validation of blockchain data structures
- Correctly validating an incorrect proof
- Incorrectly validating a correct proof
- Syncing validator set changes


