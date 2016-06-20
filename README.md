# Tendermint
Simple, Secure, Scalable Blockchain Platform

[![CircleCI](https://circleci.com/gh/tendermint/tendermint.svg?style=svg)](https://circleci.com/gh/tendermint/tendermint)

_NOTE: This is yet pre-alpha non-production-quality software._

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
* 

### Install

Make sure you have installed Go and [set the GOPATH](https://github.com/tendermint/tendermint/wiki/Setting-GOPATH).

Install `glide`, used for dependency management:

```
go get https://github.com/Masterminds/glide
```

Install tendermint:

```
mkdir -p $GOPATH/src/github.com/tendermint
git clone https://github.com/tendermint/tendermint $GOPATH/src/github.com/tendermint/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint
glide install
go install ./cmd/tendermint
```

Initialize a sample tendermint directory with an example genesis file (in `~/.tendermint`):

```
tendermint init
```

Now run the tendermint node:

```
tendermint node --proxy_app=dummy
```

For tutorials on running other applications with Tendermint, and for launching test networks,
see http://tendermint.com/guide/
