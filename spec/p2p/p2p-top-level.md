*** This is the beginning of an unfinished draft. Don't continue reading! ***

# << Tendermint P2P >>

> Rough outline of what the component is doing and why. 2-3 paragraphs 

The Tendermint consists of multiple protocols, namely,
- Consensus
- Mempool
- Evidence
- Blocksync
- Statesync

that each plays a role in making sure that validators can produce blocks. These protocols are implemented in so-called reactors (one for each protocol) that encode two functionalities:

- Protocol logic (maintaining the local state of the protocols and deciding what messages to send to others, e.g., the rules we find in the arXiv paper) 
- Message exchange with other nodes (Gossip)

As all-to-all communication is not scaling to the system sizes of typical Cosmos blockchains (e.g., 200 validator nodes + seed nodes + sentry nodes + other full nodes), the communication is restricted to an overlay network. This overlay network is established by the peer-to-peer system (p2p), which is composed of the p2p layers of the participating nodes that locally decide with which peers a node keeps connections.



PEX 

# Outline

> Table of content with rough outline for the parts

- [Part I](#part-i---tendermint-blockchain): Introduction of
 relevant terms of the Tendermint
blockchain.

- [Part II](#part-ii---sequential-definition-problem): 
    - [Informal Problem
      statement](#Informal-Problem-statement): For the general
      audience, that is, engineers who want to get an overview over what
      the component is doing from a bird's eye view.
    - [Sequential Problem statement](#Sequential-Problem-statement):
      Provides a mathematical definition of the problem statement in
      its sequential form, that is, ignoring the distributed aspect of
      the implementation of the blockchain.

- [Part III](#part-iii---as-distributed-system): Distributed
  aspects, system assumptions and temporal
  logic specifications.

  - [Incentives](#incentives): how faulty full nodes may benefit from
    misbehaving and how correct full nodes benefit from cooperating.
  
  - [Computational Model](#Computational-Model):
      timing and correctness assumptions.

  - [Distributed Problem Statement](#Distributed-Problem-Statement):
      temporal properties that formalize safety and liveness
      properties in the distributed setting.

- [Part IV](#part-iv---Protocol):
  Specification of the protocols.

     - [Definitions](#Definitions): Describes inputs, outputs,
       variables used by the protocol, auxiliary functions

     - [Protocol](#core-verification): gives an outline of the solution,
       and details of the functions used (with preconditions,
       postconditions, error conditions).

     - [Liveness Scenarios](#liveness-scenarios): when the light
       client makes progress depends heavily on the changes in the
       validator sets of the blockchain. We discuss some typical scenarios.

- [Part V](#part-v---supporting-the-ibc-relayer): Additional
  discussions and analysis


In this document we quite extensively use tags in order to be able to
reference assumptions, invariants, etc. in future communication. In
these tags we frequently use the following short forms:

- TMBC: Tendermint blockchain
- SEQ: for sequential specifications
- LCV: Lightclient Verification
- LIVE: liveness
- SAFE: safety
- FUNC: function
- INV: invariant
- A: assumption



# Part I - Tendermint Blockchain

> necessary parts of the blockchain spec. Might be replaced by a link
> to the spec once we have a published version of it.

## Context of this document

> mention other components and or specifications that are relevant for this
spec. Possible interactions, possible use cases, etc. 

> should give the reader the understanding in what environment this component
will be used. 



# Part II - Sequential Definition of the  Problem


##  Informal Problem statement

> for the general audience, that is, engineers who want to get an overview over what the component is doing
from a bird's eye view. 


## Sequential Problem statement

> should be English and precise. will be accompanied with a TLA spec.


# Part III - Distributed System

> Introduce distributed aspects 

> Timing and correctness assumptions. Possibly with justification that the
assumptions make sense, e.g., it is in the interest of a full node to behave
correctly 

> should have clear formalization in temporal logic.

## Incentives


## Computational Model

## Distributed Problem Statement

### Two Kinds of Termination

### Design choices

> input/output variables used to define the temporal properties. Most likely they come from an ADR


### Temporal Properties

> safety specifications / invariants in English 

> liveness specifications in English. Possibly with timing/fairness requirements:
e.g., if the component is connected to a correct full node and communication is
reliable and timely, then something good happens eventually. 

should have clear formalization in temporal logic.


### Solving the sequential specification

> How is the problem statement linked to the "Sequential Problem statement". 
Simulation, implementation, etc. relations 


# Part IV - Protocol

> Overview


## Definitions

### Data Types

### Inputs


### Configuration Parameters

### Variables

### Assumptions

### Invariants

### Used Remote Functions / Exchanged Messages

## <<Core Protocol>>

### Outline

> Describe solution (in English), decomposition into functions, where communication to other components happens.


### Details of the Functions

> Function signatures followed by pseudocode (optional) and a list of features (required):
> - Implementation remarks (optional)
>   - e.g. (local/remote) function called in the body of this function
> - Expected precondition
> - Expected postcondition
> - Error condition


### Solving the distributed specification

> Proof sketches of why we believe the solution satisfies the problem statement.
Possibly giving inductive invariants that can be used to prove the specifications
of the problem statement 

> In case the specification describes an existing protocol with known issues,
e.g., liveness bugs, etc. "Correctness Arguments" should be replace by
a section called "Analysis"



## Liveness Scenarios



# Part V - Additional Discussions





# References

[[block]] Specification of the block data structure. 

[[RPC]] RPC client for Tendermint

[[fork-detector]] The specification of the light client fork detector.

[[fullnode]] Specification of the full node API

[[ibc-rs]] Rust implementation of IBC modules and relayer.

[[lightclient]] The light client ADR [77d2651 on Dec 27, 2019].

[RPC]: https://docs.tendermint.com/master/rpc/

[block]: https://github.com/tendermint/spec/blob/master/spec/blockchain/blockchain.md

[TMBC-HEADER-link]: #tmbc-header.1
[TMBC-SEQ-link]: #tmbc-seq.1
[TMBC-CorrFull-link]: #tmbc-corr-full.1
[TMBC-Auth-Byz-link]: #tmbc-auth-byz.1
[TMBC-TIME_PARAMS-link]: tmbc-time-params.1
[TMBC-FM-2THIRDS-link]: #tmbc-fm-2thirds.1
[TMBC-VAL-CONTAINS-CORR-link]: tmbc-val-contains-corr.1
[TMBC-VAL-COMMIT-link]: #tmbc-val-commit.1
[TMBC-SOUND-DISTR-POSS-COMMIT-link]: #tmbc-sound-distr-poss-commit.1

[lightclient]: https://github.com/interchainio/tendermint-rs/blob/e2cb9aca0b95430fca2eac154edddc9588038982/docs/architecture/adr-002-lite-client.md
[fork-detector]: https://github.com/informalsystems/tendermint-rs/blob/master/docs/spec/lightclient/detection.md
[fullnode]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md

[ibc-rs]:https://github.com/informalsystems/ibc-rs

[FN-LuckyCase-link]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#fn-luckycase

[blockchain-validator-set]: https://github.com/tendermint/spec/blob/master/spec/blockchain/blockchain.md#data-structures
[fullnode-data-structures]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#data-structures

[FN-ManifestFaulty-link]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#fn-manifestfaulty

[arXiv]: https://arxiv.org/abs/1807.04938