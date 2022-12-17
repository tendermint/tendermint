# Quality Assurance

This document provides an overview of our quality assurance processes for
Tendermint. It maps out the various concerns we have regarding software quality
against the approaches we use to ensure quality. It also attempts to capture the
current state of our QA process and where there are still deficiencies to be
addressed.

There is a [special QA log section](#qa-log) at the end of this document where
we can capture references to one-off QA efforts.

## Approaches

We make use of the following approaches to ensure delivery of quality software,
with associated relative costs where applicable:

| Approach                  | Setup Cost   | Per-Execution Cost |
|---------------------------|--------------|--------------------|
| Code review               |              | Moderate/High      |
| Static analysis           | Low          | Low                |
| Unit testing              | Moderate     | Low                |
| Fuzz testing              | Low/Moderate | Low                |
| Model-based testing (MBT) | High         | Low                |
| End-to-end (E2E) testing  | High         | Low                |
| Testnet testing           | High         | Moderate/High      |
| Protocol audits           |              | High               |

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

## QA Log

In this section, we aim to capture all of our one-off QA efforts. Links to the
results of the following types of efforts can and should be added under the
[Log](#log) section in chronological order, with the _most recent entries
first_:

- Audit results
- One-off testnet executions
- Model checker runs
- Experiment results
- Any other manual QA effort you feel is important to share with the community

When capturing QA efforts, please use the following format:

```markdown
- YYYY-mm-dd: Descriptive title
  - A description of the QA effort.
  - References:
    - [Link 1]
    - [Link 2]
    - ...
```

For example:

```markdown
- 2022-08-22: 20-node testnet
  - A simple 20-validator testnet was run for 1 hour under a constant
    transaction load from a single source into a single validator in order to
    measure throughput. Associated Tendermint version: v0.34.21, with no
    customizations.
  - References:
    - [GitHub issue](https://github.com/tendermint/tendermint/issues/9020)
    - [Prometheus metrics database dump](https://informal-tendermint.fra1.digitaloceanspaces.com/testnets/2022-08-22/prometheus.zip)
```

### Log

(No entries at present)
