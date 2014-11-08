Tendermint in Golang

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

go build -o tendermint github.com/tendermint/tendermint/cmd

## Run

./tendermint daemon

## Contribute

## Resources
