NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

### Block
A `Block` is composed of:
- [`Header`](#header): basic information about the chain and last state
- [`Data`](#data): transactions and other data
- [`LastCommit`](#commit): +2/3 precommit signatures for the previous block

### Header
The block `Header` is composed of:
- `ChainId (string)`: name of the blockchain, e.g. "tendermint"
- `Height (int)`: sequential block number starting with 1
- `Time (time)`: local time of the proposer who proposed this block
- `LastBlockHash ([]byte)`: [`block hash`](#block-hash) of the previous block at height `Height-1` 
- `LastBlockParts (PartSetHeader)`: [`partset header`](#partset-header) of the previous block
- `StateHash ([]byte)`: [`state hash`](#state-hash) of the state after processing this block

### Data
The block `Data` is composed of:
- `Txs ([]Tx)`: a list of [`transactions`](#transaction)

### Commit
The block `Commit` is composed of:
- `Precommits ([]Vote)`: a list of [`precommit votes`](#vote)

### Vote
A `Vote` is composed of:
- `Height (int)`: The block height being decided on
- `Round (int)`: The consensus round number, starting with 0
- `Type (byte)`: The type of vote, either a `prevote` or a `precommit`
- `BlockHash ([]byte)`: The [`block hash`](#block-hash) of a valid block, or `nil`
- `BlockParts (PartSetHeader)`: The corresponding [`partset header`](#partset-header), or `x0000` if the `block hash` is `nil`
- `Signature (Signature)`: The signature of this `Vote`'s [`sign-bytes`](#vote-sign-bytes)

#### Vote Sign Bytes
The `sign-bytes` of a transaction is produced by taking a [`stable-json`](https://github.com/substack/json-stable-stringify)-like deterministic JSON [`wire`](Wire-Protocol) encoding of the vote (excluding the `Signature` field), and wrapping it with `{"chain_id":"tendermint","vote":...}`.

For example, a precommit vote might have the following `sign-bytes`:

```json
{"chain_id":"tendermint","vote":{"block_hash":"611801F57B4CE378DF1A3FFF1216656E89209A99","block_parts_header":{"hash":"B46697379DBE0774CC2C3B656083F07CA7E0F9CE","total":123},"height":1234,"round":1,"type":2}}
```

### Block Hash

The block hash is the [Simple Tree hash](Merkle-Trees#simple-tree-with-dictionaries) of the fields of the block `Header` encoded as a list of `KVPair`s.

### State Hash

The state hash is the [Simple Tree hash](Merkle-Trees#simple-tree-with-dictionaries) of the state's fields (e.g. `BondedValidators`, `UnbondingValidators`, `Accounts`, `ValidatorInfos`, and `NameRegistry`) encoded as a list of `KVPair`s.  This state hash is recursively included in the block `Header` and thus the [block hash](#block-hash) indirectly.

### Transaction

A transaction is any sequence of bytes.  It is up to your [TMSP](https://github.com/tendermint/tmsp) application to accept or reject transactions.

### PartSet

PartSet is used to split a byteslice of data into parts (pieces) for transmission.
By splitting data into smaller parts and computing a Merkle root hash on the list,
you can verify that a part is legitimately part of the complete data, and the
part can be forwarded to other peers before all the parts are known.  In short,
it's a fast way to propagate a large file over a gossip network.

PartSet was inspired by the LibSwift project.

Usage:

```Go
data := RandBytes(2 << 20) // Something large

partSet := NewPartSetFromData(data)
partSet.Total()     // Total number of 4KB parts
partSet.Count()     // Equal to the Total, since we already have all the parts
partSet.Hash()      // The Merkle root hash
partSet.BitArray()  // A BitArray of partSet.Total() 1's

header := partSet.Header() // Send this to the peer
header.Total        // Total number of parts
header.Hash         // The merkle root hash

// Now we'll reconstruct the data from the parts
partSet2 := NewPartSetFromHeader(header)
partSet2.Total()    // Same total as partSet.Total()
partSet2.Count()    // Zero, since this PartSet doesn't have any parts yet.
partSet2.Hash()     // Same hash as in partSet.Hash()
partSet2.BitArray() // A BitArray of partSet.Total() 0's

// In a gossip network the parts would arrive in arbitrary order, perhaps
// in response to explicit requests for parts, or optimistically in response
// to the receiving peer's partSet.BitArray().
for !partSet2.IsComplete() {
    part := receivePartFromGossipNetwork()
    added, err := partSet2.AddPart(part)
    if err != nil {
		// A wrong part,
        // the merkle trail does not hash to partSet2.Hash()
    } else if !added {
        // A duplicate part already received
    }
}

data2, _ := ioutil.ReadAll(partSet2.GetReader())
bytes.Equal(data, data2) // true
```

