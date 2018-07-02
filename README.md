# Tendermint

[Byzantine-Fault Tolerant](https://en.wikipedia.org/wiki/Byzantine_fault_tolerance)
[State Machine Replication](https://en.wikipedia.org/wiki/State_machine_replication).
Or [Blockchain](https://en.wikipedia.org/wiki/Blockchain_(database)) for short.

[![version](https://img.shields.io/github/tag/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/releases/latest)
[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://godoc.org/github.com/tendermint/tendermint)
[![Go version](https://img.shields.io/badge/go-1.9.2-blue.svg)](https://github.com/moovweb/gvm)
[![riot.im](https://img.shields.io/badge/riot.im-JOIN%20CHAT-green.svg)](https://riot.im/app/#/room/#tendermint:matrix.org)
[![license](https://img.shields.io/github/license/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/blob/master/LICENSE)
[![](https://tokei.rs/b1/github/tendermint/tendermint?category=lines)](https://github.com/tendermint/tendermint)


Branch    | Tests | Coverage
----------|-------|----------
master    | [![CircleCI](https://circleci.com/gh/tendermint/tendermint/tree/master.svg?style=shield)](https://circleci.com/gh/tendermint/tendermint/tree/master) | [![codecov](https://codecov.io/gh/tendermint/tendermint/branch/master/graph/badge.svg)](https://codecov.io/gh/tendermint/tendermint)
develop   | [![CircleCI](https://circleci.com/gh/tendermint/tendermint/tree/develop.svg?style=shield)](https://circleci.com/gh/tendermint/tendermint/tree/develop) | [![codecov](https://codecov.io/gh/tendermint/tendermint/branch/develop/graph/badge.svg)](https://codecov.io/gh/tendermint/tendermint)

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state transition machine - written in any programming language -
and securely replicates it on many machines.

For protocol details, see [the specification](/docs/spec).

## A Note on Production Readiness

While Tendermint is being used in production in private, permissioned
environments, we are still working actively to harden and audit it in preparation
for use in public blockchains, such as the [Cosmos Network](https://cosmos.network/).
We are also still making breaking changes to the protocol and the APIs.
Thus we tag the releases as *alpha software*.

In any case, if you intend to run Tendermint in production,
please [contact us](https://riot.im/app/#/room/#tendermint:matrix.org) :)

## Security

To report a security vulnerability, see our [bug bounty
program](https://tendermint.com/security).

For examples of the kinds of bugs we're looking for, see [SECURITY.md](SECURITY.md)

## Minimum requirements

Requirement|Notes
---|---
Go version | Go1.9 or higher

## Install

See the [install instructions](/docs/install.md)

## Quick Start

- [Single node](/docs/using-tendermint.md)
- [Local cluster using docker-compose](/networks/local)
- [Remote cluster using terraform and ansible](/docs/terraform-and-ansible.md)
- [Join the public testnet](https://cosmos.network/testnet)

## Resources

### Tendermint Core

For details about the blockchain data structures and the p2p protocols, see the
the [Tendermint specification](/docs/spec).

For details on using the software, [Read The Docs](https://tendermint.readthedocs.io/en/master/).
Additional information about some - and eventually all - of the sub-projects below, can be found at Read The Docs.


### Sub-projects

* [Amino](http://github.com/tendermint/go-amino), a reflection-based improvement on proto3
* [IAVL](http://github.com/tendermint/iavl), Merkleized IAVL+ Tree implementation

### Tools
* [Deployment, Benchmarking, and Monitoring](http://tendermint.readthedocs.io/projects/tools/en/develop/index.html#tendermint-tools)

### Applications

* [Cosmos SDK](http://github.com/cosmos/cosmos-sdk); a cryptocurrency application framework
* [Ethermint](http://github.com/tendermint/ethermint); Ethereum on Tendermint
* [Many more](https://tendermint.readthedocs.io/en/master/ecosystem.html#abci-applications)

### More

* [Master's Thesis on Tendermint](https://atrium.lib.uoguelph.ca/xmlui/handle/10214/9769)
* [Original Whitepaper](https://tendermint.com/static/docs/tendermint.pdf)
* [Tendermint Blog](https://blog.cosmos.network/tendermint/home)
* [Cosmos Blog](https://blog.cosmos.network)

## Contributing

Yay open source! Please see our [contributing guidelines](CONTRIBUTING.md).

## Versioning

### SemVer

Tendermint uses [SemVer](http://semver.org/) to determine when and how the version changes.
According to SemVer, anything in the public API can change at any time before version 1.0.0

To provide some stability to Tendermint users in these 0.X.X days, the MINOR version is used
to signal breaking changes across a subset of the total public API. This subset includes all
interfaces exposed to other processes (cli, rpc, p2p, etc.), but does not
include the in-process Go APIs.

That said, breaking changes in the following packages will be documented in the
CHANGELOG even if they don't lead to MINOR version bumps:

- types
- rpc/client
- config
- node

Exported objects in these packages that are not covered by the versioning scheme
are explicitly marked by `// UNSTABLE` in their go doc comment and may change at any
time without notice. Functions, types, and values in any other package may also change at any time.

### Upgrades

In an effort to avoid accumulating technical debt prior to 1.0.0,
we do not guarantee that breaking changes (ie. bumps in the MINOR version)
will work with existing tendermint blockchains. In these cases you will
have to start a new blockchain, or write something custom to get the old
data into the new chain.

However, any bump in the PATCH version should be compatible with existing histories
(if not please open an [issue](https://github.com/tendermint/tendermint/issues)).

## Code of Conduct

Please read, understand and adhere to our [code of conduct](CODE_OF_CONDUCT.md).
