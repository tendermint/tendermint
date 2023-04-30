# Tendermint

_UPDATE: TendermintCore featureset is frozen for LTS, see issue https://github.com/tendermint/tendermint/issues/9972_<br/>
_This is the latest stable release used by cosmoshub-4, version 0.34.24_<br/>
_The previous main branch (v0.38.xx) can now be found under "main_backup"_<br/>

![banner](docs/tendermint-core-image.jpg)

[Byzantine-Fault Tolerant][bft] [State Machine Replication][smr]. Or
[Blockchain], for short.

[![Version][version-badge]][version-url]
[![API Reference][api-badge]][api-url]
[![Go version][go-badge]][go-url]
[![Discord chat][discord-badge]][discord-url]
[![License][license-badge]][license-url]
[![Sourcegraph][sg-badge]][sg-url]

| Branch | Tests                              | Linting                         |
|--------|------------------------------------|---------------------------------|
| main   | [![Tests][tests-badge]][tests-url] | [![Lint][lint-badge]][lint-url] |

Tendermint Core is a Byzantine Fault Tolerant (BFT) middleware that takes a
state transition machine - written in any programming language - and securely
replicates it on many machines.

For protocol details, refer to the [Tendermint Specification](./spec/README.md).

For detailed analysis of the consensus protocol, including safety and liveness
proofs, read our paper, "[The latest gossip on BFT
consensus](https://arxiv.org/abs/1807.04938)".

## Documentation

Complete documentation can be found on the
[website](https://docs.tendermint.com/).

## Releases

Please do not depend on `main` as your production branch. Use
[releases](https://github.com/tendermint/tendermint/releases) instead.

Tendermint has been in the production of private and public environments, most
notably the blockchains of the Cosmos Network. we haven't released v1.0 yet
since we are making breaking changes to the protocol and the APIs. See below for
more details about [versioning](#versioning).

In any case, if you intend to run Tendermint in production, we're happy to help.
You can contact us [over email](mailto:hello@interchain.io) or [join the
chat](https://discord.gg/cosmosnetwork).

More on how releases are conducted can be found [here](./RELEASES.md).

## Security

To report a security vulnerability, see our [bug bounty
program](https://hackerone.com/cosmos). For examples of the kinds of bugs we're
looking for, see [our security policy](SECURITY.md).

We also maintain a dedicated mailing list for security updates. We will only
ever use this mailing list to notify you of vulnerabilities and fixes in
Tendermint Core. You can subscribe [here](http://eepurl.com/gZ5hQD).

## Minimum requirements

| Requirement | Notes             |
|-------------|-------------------|
| Go version  | Go 1.18 or higher |

### Install

See the [install instructions](./docs/introduction/install.md).

### Quick Start

- [Single node](./docs/introduction/quick-start.md)
- [Local cluster using docker-compose](./docs/tools/docker-compose.md)
- [Remote cluster using Terraform and Ansible](./docs/tools/terraform-and-ansible.md)

## Contributing

Please abide by the [Code of Conduct](CODE_OF_CONDUCT.md) in all interactions.

Before contributing to the project, please take a look at the [contributing
guidelines](CONTRIBUTING.md) and the [style guide](STYLE_GUIDE.md). You may also
find it helpful to read the [specifications](./spec/README.md), and familiarize
yourself with our [Architectural Decision Records
(ADRs)](./docs/architecture/README.md) and
[Request For Comments (RFCs)](./docs/rfc/README.md).

## Versioning

### Semantic Versioning

Tendermint uses [Semantic Versioning](http://semver.org/) to determine when and
how the version changes. According to SemVer, anything in the public API can
change at any time before version 1.0.0

To provide some stability to users of 0.X.X versions of Tendermint, the MINOR
version is used to signal breaking changes across Tendermint's API. This API
includes all publicly exposed types, functions, and methods in non-internal Go
packages as well as the types and methods accessible via the Tendermint RPC
interface.

Breaking changes to these public APIs will be documented in the CHANGELOG.

### Upgrades

In an effort to avoid accumulating technical debt prior to 1.0.0, we do not
guarantee that breaking changes (ie. bumps in the MINOR version) will work with
existing Tendermint blockchains. In these cases you will have to start a new
blockchain, or write something custom to get the old data into the new chain.
However, any bump in the PATCH version should be compatible with existing
blockchain histories.

For more information on upgrading, see [UPGRADING.md](./UPGRADING.md).

### Supported Versions

Because we are a small core team, we only ship patch updates, including security
updates, to the most recent minor release and the second-most recent minor
release. Consequently, we strongly recommend keeping Tendermint up-to-date.
Upgrading instructions can be found in [UPGRADING.md](./UPGRADING.md).

### Firehose

Firehose documentation can be found in [FIREHOSE.md](./FIREHOSE.md)

## Resources

### Libraries

- [Cosmos SDK](http://github.com/cosmos/cosmos-sdk); A framework for building
  applications in Golang
- [Tendermint in Rust](https://github.com/informalsystems/tendermint-rs)
- [ABCI Tower](https://github.com/penumbra-zone/tower-abci)

### Applications

- [Cosmos Hub](https://hub.cosmos.network/)
- [Terra](https://www.terra.money/)
- [Celestia](https://celestia.org/)
- [Anoma](https://anoma.network/)
- [Vocdoni](https://docs.vocdoni.io/)

### Research

- [The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)
- [Master's Thesis on Tendermint](https://atrium.lib.uoguelph.ca/xmlui/handle/10214/9769)
- [Original Whitepaper: "Tendermint: Consensus Without Mining"](https://tendermint.com/static/docs/tendermint.pdf)
- [Tendermint Core Blog](https://medium.com/tendermint/tagged/tendermint-core)
- [Cosmos Blog](https://blog.cosmos.network/tendermint/home)

## Join us!

Tendermint Core is maintained by [Interchain GmbH](https://interchain.berlin).
If you'd like to work full-time on Tendermint Core,
[we're hiring](https://interchain-gmbh.breezy.hr/)!

Funding for Tendermint Core development comes primarily from the
[Interchain Foundation](https://interchain.io), a Swiss non-profit. The
Tendermint trademark is owned by [Tendermint Inc.](https://tendermint.com), the
for-profit entity that also maintains [tendermint.com](https://tendermint.com).

[bft]: https://en.wikipedia.org/wiki/Byzantine_fault_tolerance
[smr]: https://en.wikipedia.org/wiki/State_machine_replication
[Blockchain]: https://en.wikipedia.org/wiki/Blockchain
[version-badge]: https://img.shields.io/github/tag/tendermint/tendermint.svg
[version-url]: https://github.com/tendermint/tendermint/releases/latest
[api-badge]: https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
[api-url]: https://pkg.go.dev/github.com/tendermint/tendermint
[go-badge]: https://img.shields.io/badge/go-1.18-blue.svg
[go-url]: https://github.com/moovweb/gvm
[discord-badge]: https://img.shields.io/discord/669268347736686612.svg
[discord-url]: https://discord.gg/cosmosnetwork
[license-badge]: https://img.shields.io/github/license/tendermint/tendermint.svg
[license-url]: https://github.com/tendermint/tendermint/blob/main/LICENSE
[sg-badge]: https://sourcegraph.com/github.com/tendermint/tendermint/-/badge.svg
[sg-url]: https://sourcegraph.com/github.com/tendermint/tendermint?badge
[tests-url]: https://github.com/tendermint/tendermint/actions/workflows/tests.yml
[tests-badge]: https://github.com/tendermint/tendermint/actions/workflows/tests.yml/badge.svg?branch=main
[lint-badge]: https://github.com/tendermint/tendermint/actions/workflows/lint.yml/badge.svg
[lint-url]: https://github.com/tendermint/tendermint/actions/workflows/lint.yml
