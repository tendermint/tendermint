# Tendermint
Simple, Secure, Scalable Blockchain Platform

[![version](https://img.shields.io/github/tag/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/releases/latest)
[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://godoc.org/github.com/tendermint/tendermint)
[![chat](https://img.shields.io/badge/slack-join%20chat-pink.svg)](http://forum.tendermint.com:3000/)
[![license](https://img.shields.io/github/license/tendermint/tendermint.svg)](https://github.com/tendermint/tendermint/blob/master/LICENSE)

Branch    | Tests | Coverage | Report Card
----------|-------|----------|-------------
develop   | [![CircleCI](https://circleci.com/gh/tendermint/tendermint/tree/develop.svg?style=shield)](https://circleci.com/gh/tendermint/tendermint/tree/develop) | [![codecov](https://codecov.io/gh/tendermint/tendermint/branch/develop/graph/badge.svg)](https://codecov.io/gh/tendermint/tendermint) | [![Go Report Card](https://goreportcard.com/badge/github.com/tendermint/tendermint/tree/develop)](https://goreportcard.com/report/github.com/tendermint/tendermint/tree/develop)
master    | [![CircleCI](https://circleci.com/gh/tendermint/tendermint/tree/master.svg?style=shield)](https://circleci.com/gh/tendermint/tendermint/tree/master) | [![codecov](https://codecov.io/gh/tendermint/tendermint/branch/master/graph/badge.svg)](https://codecov.io/gh/tendermint/tendermint) | [![Go Report Card](https://goreportcard.com/badge/github.com/tendermint/tendermint/tree/master)](https://goreportcard.com/report/github.com/tendermint/tendermint/tree/master)

_NOTE: This is yet pre-alpha non-production-quality software._

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state transition machine, written in any programming language,
and replicates it on many machines.
See the [application developers guide](https://github.com/tendermint/tendermint/wiki/Application-Developers) to get started.

## Contributing

Yay open source! Please see our [contributing guidelines](https://github.com/tendermint/tendermint/wiki/Contributing).

## Resources

### Tendermint Core

- [Introduction](https://github.com/tendermint/tendermint/wiki/Introduction)
- [Validators](https://github.com/tendermint/tendermint/wiki/Validators)
- [Byzantine Consensus Algorithm](https://github.com/tendermint/tendermint/wiki/Byzantine-Consensus-Algorithm)
- [Block Structure](https://github.com/tendermint/tendermint/wiki/Block-Structure)
- [RPC](https://github.com/tendermint/tendermint/wiki/RPC)
- [Genesis](https://github.com/tendermint/tendermint/wiki/Genesis)
- [Configuration](https://github.com/tendermint/tendermint/wiki/Configuration)
- [Light Client Protocol](https://github.com/tendermint/tendermint/wiki/Light-Client-Protocol)
- [Roadmap for V2](https://github.com/tendermint/tendermint/wiki/Roadmap-for-V2)

### Sub-projects

* [TMSP](http://github.com/tendermint/tmsp)
* [Mintnet](http://github.com/tendermint/mintnet)
* [Go-Wire](http://github.com/tendermint/go-wire)
* [Go-P2P](http://github.com/tendermint/go-p2p)
* [Go-Merkle](http://github.com/tendermint/go-merkle)

## Install

`go get -u github.com/tendermint/tendermint/cmd/tendermint`

For more details, see the [install guide](https://github.com/tendermint/tendermint/wiki/Installation).
