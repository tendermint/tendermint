TenderMint - proof of concept

* **[p2p](https://github.com/tendermint/tendermint/blob/master/p2p):**  P2P networking stack.  Designed to be extensible.
* **[merkle](https://github.com/tendermint/tendermint/blob/master/merkle):** Immutable Persistent Merkle-ized AVL+ Tree, used primarily for keeping track of mutable state like account balances.
* **[blocks](https://github.com/tendermint/tendermint/blob/master/blocks):** The blockchain, storage of blocks, and all the associated structures.
* **[state](https://github.com/tendermint/tendermint/blob/master/state):** The application state, which is mutated by blocks in the blockchain.
* **[consensus](https://github.com/tendermint/tendermint/blob/master/consensus):** The core consensus algorithm logic.
* **[mempool](https://github.com/tendermint/tendermint/blob/master/mempool):** Handles the broadcasting of uncommitted transactions.
* **[crypto](https://github.com/tendermint/tendermint/blob/master/crypto):** Includes cgo bindings of ed25519.

### Status

* Mempool *now*
* Consensus *complete*
* Block propagation *sidelined*
* Node & testnet *complete*
* PEX peer exchange *complete*
* p2p/* *complete*
* Ed25519 bindings *complete*
* merkle/* *complete*
