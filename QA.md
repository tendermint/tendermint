# Quality Assurance

This document provides an overview of our quality assurance processes for
Tendermint. It maps out the various concerns we have regarding software quality
against the approaches we use to ensure quality. It also attempts to capture the
current state of our QA process and where there are still deficiencies to be
addressed.

## Approaches

We make use of the following approaches to ensure delivery of quality software,
with associated relative costs where applicable:

| Approach                  | Implementation Cost | Execution Cost |
|---------------------------|---------------------|----------------|
| Code review               |                     | Moderate/High  |
| Static analysis           | Low                 | Low            |
| Unit testing              | Moderate            | Low            |
| Fuzz testing              | Low/Moderate        | Low            |
| Model-based testing (MBT) | High                | Low            |
| End-to-end (E2E) testing  | High                | Low            |
| Testnet testing           | High                | Moderate/High  |
| Protocol audits           |                     | High           |

## Concerns

Tendermint QA is split across the following concerns:
- [Correct software engineering](#correct-software-engineering)
- Protocol correctness
  - [Consensus](#consensus)
  - [Mempool](#mempool)
  - [Evidence](#evidence)
  - [Peer exchange (PEX)](#peer-exchange-pex)
  - [Block sync](#block-sync)
  - [State sync](#state-sync)
  - [Light client](#light-client)
- [Application integration (ABCI)](#application-integration-abci)
- [Network stability](#network-stability)
- Operational dynamics
  - [Crash/recovery](#crash-recovery)
  - [Upgrading](#upgrading)

### Correct software engineering

| Concern                         | Code Review | Static Analysis | Unit Testing | Fuzz Testing | E2E Testing | Testnet Testing |
|---------------------------------|-------------|-----------------|--------------|--------------|-------------|-----------------|
| Security                        |             |                 |              |              |             |                 |
| Memory leaks                    |             |                 |              |              |             |                 |
| Storage leaks                   |             |                 |              |              |             |                 |
| Error handling                  |             |                 |              |              |             |                 |
| Serialization / deserialization |             |                 |              |              |             |                 |

### Protocol correctness

#### Consensus

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |

#### Mempool

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |

#### Evidence

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |

#### Peer exchange (PEX)

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |

#### Block sync

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |

#### State sync

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |

#### Light client

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |

### Application integration (ABCI)

| Concern                          | Code Review | Static Analysis | Unit Testing | Fuzz Testing | MBT | E2E Testing | Testnet Testing | Protocol Audit |
|----------------------------------|-------------|-----------------|--------------|--------------|-----|-------------|-----------------|----------------|
| Code/specification alignment     |             |                 |              |              |     |             |                 |                |
| - Grammar                        |             |                 |              |              |     |             |                 |                |
| - Parameters of API functions    |             |                 |              |              |     |             |                 |                |
| Integration with other protocols |             |                 |              |              |     |             |                 |                |
| Performance                      |             |                 |              |              |     |             |                 |                |
| Robustness                       |             |                 |              |              |     |             |                 |                |
| - Safe under perturbations       |             |                 |              |              |     |             |                 |                |
| - Performant after perturbations |             |                 |              |              |     |             |                 |                |
| Correct usage upholds liveness   |             |                 |              |              |     |             |                 |                |

### Network stability

### Operational dynamics

#### Crash/recovery

#### Upgrading
