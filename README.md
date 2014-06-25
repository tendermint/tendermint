TenderMint - proof of concept

* **peer:**  P2P networking stack.  Designed to be extensible.  [docs](https://github.com/tendermint/tendermint/peer/README.md)
* **merkle:** Immutable Persistent Merkle-ized AVL+ Tree, used primarily for keeping track of mutable state like account balances. [docs](https://github.com/tendermint/tendermint/blob/master/merkle/README.md)
* **crypto:** Includes cgo bindings of ed25519. [docs](https://github.com/tendermint/tendermint/blob/master/crypto/README.md)

### Status

* Still implementing peer/*
* Ed25519 bindings *complete*
* merkle/* *complete*
