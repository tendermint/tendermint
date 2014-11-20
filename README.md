[**Website**](http://tendermint.com) **|** 
[**Blog**](http://tendermint.com/posts/) **|**
[**Forum**] (http://forum.tendermint.com) **|**
[**IRC**] (http://webchat.freenode.net/?randomnick=1&channels=%23tendermint)

Tendermint in Golang

Tendermint is a completely decentralized byzantine consensus protocol suitable for use in cryptocurrencies.

This project is a reference implementation of the protocol.

## Submodules

* **[consensus](https://github.com/tendermint/tendermint/blob/master/consensus):** core consensus algorithm
* **[state](https://github.com/tendermint/tendermint/blob/master/state):** application state; mutated by transactions
* **[blocks](https://github.com/tendermint/tendermint/blob/master/blocks):** structures of the blockchain
* **[mempool](https://github.com/tendermint/tendermint/blob/master/mempool):** gossip of new transactions
* **[merkle](https://github.com/tendermint/tendermint/blob/master/merkle):** merkle hash trees
* **[p2p](https://github.com/tendermint/tendermint/blob/master/p2p):**  extensible P2P networking

## Requirements

[Go](http://golang.org) 1.2 or newer.

## Build

[![Build Status](https://drone.io/github.com/tendermint/tendermint/status.png)](https://drone.io/github.com/tendermint/tendermint/latest)

```
go get github.com/tendermint/tendermint/...
go build -o tendermint github.com/tendermint/tendermint/cmd
```

## Run

`./tendermint daemon`

## Resources

IRC Channel: #tendermint on freenode
