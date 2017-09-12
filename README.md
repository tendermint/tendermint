# Tendermint

[Byzantine-Fault Tolerant](https://en.wikipedia.org/wiki/Byzantine_fault_tolerance)
[State Machine Replication](https://en.wikipedia.org/wiki/State_machine_replication).
Or [Blockchain](https://en.wikipedia.org/wiki/Blockchain_(database)) for short.

[![version](https://img.shields.io/github/tag/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/releases/latest)
[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://godoc.org/github.com/tendermint/tendermint)
[![Rocket.Chat](https://demo.rocket.chat/images/join-chat.svg)](https://cosmos.rocket.chat/)
[![license](https://img.shields.io/github/license/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/blob/master/LICENSE)
[![](https://tokei.rs/b1/github/tendermint/tendermint?category=lines)](https://github.com/tendermint/tendermint)


Branch    | Tests | Coverage | Report Card
----------|-------|----------|-------------
develop   | [![CircleCI](https://circleci.com/gh/tendermint/tendermint/tree/develop.svg?style=shield)](https://circleci.com/gh/tendermint/tendermint/tree/develop) | [![codecov](https://codecov.io/gh/tendermint/tendermint/branch/develop/graph/badge.svg)](https://codecov.io/gh/tendermint/tendermint) | [![Go Report Card](https://goreportcard.com/badge/github.com/tendermint/tendermint/tree/develop)](https://goreportcard.com/report/github.com/tendermint/tendermint/tree/develop)
master    | [![CircleCI](https://circleci.com/gh/tendermint/tendermint/tree/master.svg?style=shield)](https://circleci.com/gh/tendermint/tendermint/tree/master) | [![codecov](https://codecov.io/gh/tendermint/tendermint/branch/master/graph/badge.svg)](https://codecov.io/gh/tendermint/tendermint) | [![Go Report Card](https://goreportcard.com/badge/github.com/tendermint/tendermint/tree/master)](https://goreportcard.com/report/github.com/tendermint/tendermint/tree/master)

_NOTE: This is alpha software. Please contact us if you intend to run it in production._

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state transition machine, written in any programming language,
and securely replicates it on many machines.

For more background, see the [introduction](https://tendermint.com/intro).

To get started developing applications, see the [application developers guide](https://tendermint.com/docs/guides/app-development).

### Code of Conduct
Please read, understand and adhere to our [code of conduct](CODE_OF_CONDUCT.md).

## Install

To download pre-built binaries, see our [downloads page](https://tendermint.com/downloads).

To install from source, you should be able to:

`go get -u github.com/tendermint/tendermint/cmd/tendermint`

For more details (or if it fails), see the [install guide](https://tendermint.com/docs/guides/install-from-source).

## Contributing

Yay open source! Please see our [contributing guidelines](CONTRIBUTING.md).

## Resources

### Tendermint Core

- [Introduction](https://tendermint.com/intro)
- [Docs](https://tendermint.com/docs)
- [Software using Tendermint](https://tendermint.com/ecosystem)

### Sub-projects

* [ABCI](http://github.com/tendermint/abci), the Application Blockchain Interface
* [Go-Wire](http://github.com/tendermint/go-wire), a deterministic serialization library
* [Go-Crypto](http://github.com/tendermint/go-crypto), an elliptic curve cryptography library
* [TmLibs](http://github.com/tendermint/tmlibs), an assortment of Go libraries
* [Merkleeyes](http://github.com/tendermint/merkleeyes), a balanced, binary Merkle tree for ABCI apps

### Tools
* [Deployment, Benchmarking, and Monitoring](https://github.com/tendermint/tools)

### Applications

* [Ethermint](http://github.com/tendermint/ethermint): Ethereum on Tendermint
* [Basecoin](http://github.com/tendermint/basecoin), a cryptocurrency application framework

### More

* [Tendermint Blog](https://blog.cosmos.network/tendermint/home)
* [Cosmos Blog](https://blog.cosmos.network)
* [Original Whitepaper (out-of-date)](http://www.the-blockchain.com/docs/Tendermint%20Consensus%20without%20Mining.pdf)
* [Master's Thesis on Tendermint](https://atrium.lib.uoguelph.ca/xmlui/handle/10214/9769)
