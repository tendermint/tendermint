# Tendermint

[Byzantine-Fault Tolerant](https://en.wikipedia.org/wiki/Byzantine_fault_tolerance)
[State Machines](https://en.wikipedia.org/wiki/State_machine_replication).
Or [Blockchain](https://en.wikipedia.org/wiki/Blockchain_(database)), for short.

[![version](https://img.shields.io/github/tag/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/releases/latest)
[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://godoc.org/github.com/tendermint/tendermint)
[![Go version](https://img.shields.io/badge/go-1.10.4-blue.svg)](https://github.com/moovweb/gvm)
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

For detailed analysis of the consensus protocol, including safety and liveness proofs,
see our recent paper, "[The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)".

## A Note on Production Readiness

While Tendermint is being used in production in private, permissioned
environments, we are still working actively to harden and audit it in preparation
for use in public blockchains, such as the [Cosmos Network](https://cosmos.network/).
We are also still making breaking changes to the protocol and the APIs.
Thus, we tag the releases as *alpha software*.

In any case, if you intend to run Tendermint in production,
please [contact us](mailto:partners@tendermint.com) and [join the chat](https://riot.im/app/#/room/#tendermint:matrix.org).

## Security

To report a security vulnerability, see our [bug bounty
program](https://hackerone.com/tendermint)

For examples of the kinds of bugs we're looking for, see [SECURITY.md](SECURITY.md)

## Minimum requirements

Requirement|Notes
---|---
Go version | Go1.11.4 or higher

## Documentation

Complete documentation can be found on the [website](https://tendermint.com/docs/).

### Install

See the [install instructions](/docs/introduction/install.md)

### Quick Start

- [Single node](/docs/introduction/quick-start.md)
- [Local cluster using docker-compose](/docs/networks/docker-compose.md)
- [Remote cluster using terraform and ansible](/docs/networks/terraform-and-ansible.md)
- [Join the Cosmos testnet](https://cosmos.network/testnet)

## Contributing

Please abide by the [Code of Conduct](CODE_OF_CONDUCT.md) in all interactions,
and the [contributing guidelines](CONTRIBUTING.md) when submitting code.

Join the larger community on the [forum](https://forum.cosmos.network/) and the [chat](https://riot.im/app/#/room/#tendermint:matrix.org).

To learn more about the structure of the software, watch the [Developer
Sessions](https://www.youtube.com/playlist?list=PLdQIb0qr3pnBbG5ZG-0gr3zM86_s8Rpqv)
and read some [Architectural
Decision Records](https://github.com/tendermint/tendermint/tree/master/docs/architecture).

Learn more by reading the code and comparing it to the
[specification](https://github.com/tendermint/tendermint/tree/develop/docs/spec).

## Versioning

### Semantic Versioning

Tendermint uses [Semantic Versioning](http://semver.org/) to determine when and how the version changes.
According to SemVer, anything in the public API can change at any time before version 1.0.0

To provide some stability to Tendermint users in these 0.X.X days, the MINOR version is used
to signal breaking changes across a subset of the total public API. This subset includes all
interfaces exposed to other processes (cli, rpc, p2p, etc.), but does not
include the in-process Go APIs.

That said, breaking changes in the following packages will be documented in the
CHANGELOG even if they don't lead to MINOR version bumps:

- crypto
- types
- rpc/client
- config
- node
- libs
  - bech32
  - common
  - db
  - errors
  - log

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

For more information on upgrading, see [UPGRADING.md](./UPGRADING.md)

## Resources

### Tendermint Core

For details about the blockchain data structures and the p2p protocols, see the
[Tendermint specification](/docs/spec).

For details on using the software, see the [documentation](/docs/) which is also
hosted at: https://tendermint.com/docs/

### Tools

Benchmarking and monitoring is provided by `tm-bench` and `tm-monitor`, respectively.
Their code is found [here](/tools) and these binaries need to be built seperately.
Additional documentation is found [here](/docs/tools).

### Sub-projects

* [Amino](http://github.com/tendermint/go-amino), reflection-based proto3, with
  interfaces
* [IAVL](http://github.com/tendermint/iavl), Merkleized IAVL+ Tree implementation

### Applications

* [Cosmos SDK](http://github.com/cosmos/cosmos-sdk); a cryptocurrency application framework
* [Ethermint](http://github.com/cosmos/ethermint); Ethereum on Tendermint
* [Many more](https://tendermint.com/ecosystem)

### Research

* [The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)
* [Master's Thesis on Tendermint](https://atrium.lib.uoguelph.ca/xmlui/handle/10214/9769)
* [Original Whitepaper](https://tendermint.com/static/docs/tendermint.pdf)
* [Blog](https://blog.cosmos.network/tendermint/home)

