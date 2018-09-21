NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

Light-client syncing of validator-changes
=========================================

Let's take a look at the current Block Header structure.

```golang
type Header struct {
	ChainID        string    `json:"chain_id"`
	Height         int       `json:"height"`
	Time           time.Time `json:"time"`
	NumTxs         int       `json:"num_txs"` // XXX: Can we get rid of this?
	LastBlockID    BlockID   `json:"last_block_id"`
	LastCommitHash []byte    `json:"last_commit_hash"` // commit from validators from the last block
	DataHash       []byte    `json:"data_hash"`        // transactions
	ValidatorsHash []byte    `json:"validators_hash"`  // validators for the current block
	AppHash        []byte    `json:"app_hash"`         // state after txs from the previous block
}
```


Gradual changes
---------------

This is assuming that there are many validators, and that the total amount of voting power that changes per block is small (e.g. << 1/3 of the voting power) and that the change is gradual.

Lets say we have the validator set V(H) for block H.  That is, +2/3 of V(H) have signed block H's blockhash.  Lets say the light client trusts block H.  As long as the validator-set changes are gradual, for many subsequent blocks, even though the validator set is not exactly V(H), we will be able to find a sufficinet number of signatures for block H+x (x > 0) where the signatures come from the set V(H).  What is a sufficient number of signatures?  Well, it only needs to be 1/3+ because these validators are signing blocks at H+x whose Header includes the ValidatorsHash hash.  While 1/3+ signatures isn't enough to make a commit, e.g. they may move on to subsequent rounds and actually commit another block, all valid proposals for block H+x should include the same identical ValidatorsHash because it was determined by the previous transactions that were committed at block H+x-1 and prior.

So, we can find the largest x (where x < unbonding period) such that the light client is convinced of the new validator set.  And in this way the light-client can fast-forward, and as long as the validator-set changes are very gradual, the light client need only sync the validator set a few times, and still get the 1/3+ economic guarantee.  This is assuming that validators are slashed for signing a block that includes an invalid header.ValidatorsHash.  (If we don't assume this we can do something else, but more on that later)


Sudden changes
--------------

This is about when the validator-set can change dramatically from 1 block to another.

Header.ValidatorsHash represents the validator set that is responsible for signing that block.

Lets say we have the Commit for a block.  Now, the transactions (Merkleized into DataHash) of this block haven't been executed yet, so we haven't gotten the result of EndBlock which determines who the next validators will be.  But after the execution of this block, EndBlock will return validator diffs, and the next block will need to be signed by the updated validator set. (according to the current implementation of Tendermint)

This is unfortunate, because what we really want is something like this:

```golang
type Header struct {
	...
	ValidatorsHash []byte        `json:"validators_hash"`  // validators for the current block
	NextValidatorsHash []byte    `json:"validators_hash"`  // validators for the next block
	AppHash        []byte        `json:"app_hash"`         // state after txs from the previous block
}
```

Then we can look at the Commit for the block.Header, which would be signed by +2/3 of the validators represented by ValidatorsHash, and a light client can know how the validator-set is to evolve for the next block.

Maybe we can delay the effect of EndBlock>validator diffs for 1 block, and thereby allow us to include this NextValidatorsHash.  Are there other good solutions?

Note to self: I think we want to make this change anyways in order to allow for lightweight validator signing devices that can always update its locally cached validator-set, and ensure that it never signs a block with an invalid header.ValidatorsHash, to enforce slashing for signing a block with the wrong header.ValidatorsHash.