Tendermint in Golang

[**Website**](http://tendermint.com) **|** 
[**Blog**](http://tendermint.com/posts/) **|**
[**Forum**] (http://forum.tendermint.com) **|**
**IRC:** #tendermint@freenode

Tendermint is a completely decentralized byzantine consensus protocol suitable for use in crypto-currencies.

This project is a reference implementation of the protocol.

## Submodules

* **[consensus](https://github.com/tendermint/tendermint/blob/master/consensus):** core consensus algorithm
* **[state](https://github.com/tendermint/tendermint/blob/master/state):** application state; mutated by transactions
* **[blocks](https://github.com/tendermint/tendermint/blob/master/blocks):** structures of the blockchain
* **[mempool](https://github.com/tendermint/tendermint/blob/master/mempool):** gossip of new transactions
* **[merkle](https://github.com/tendermint/tendermint/blob/master/merkle):** merkle hash trees
* **[p2p](https://github.com/tendermint/tendermint/blob/master/p2p):**  extensible P2P networking

## Build

[![Build Status](https://drone.io/github.com/tendermint/tendermint/status.png)](https://drone.io/github.com/tendermint/tendermint/latest)

`go build -o tendermint github.com/tendermint/tendermint/cmd`

## Run

`./tendermint daemon`

## Contribute

## Resources
