# RFC {RFC-NUMBER}: {TITLE}

## Changelog

- {date}: {changelog}

## Abstract

> A brief high-level synopsis of the topic of discussion for this RFC, ideally
> just a few sentences.  This should help the reader quickly decide whether the
> rest of the discussion is relevant to their interest.

## Background

> Any context or orientation needed for a reader to understand and participate
> in the substance of the Discussion. If necessary, this section may include
> links to other documentation or sources rather than restating existing
> material, but should provide enough detail that the reader can tell what they
> need to read to be up-to-date.

### References

> Links to external materials needed to follow the discussion may be added here.
>
> In addition, if the discussion in a request for comments leads to any design
> decisions, it may be helpful to add links to the ADR documents here after the
> discussion has settled.

## Discussion

> This section contains the core of the discussion.
>
> There is no fixed format for this section, but ideally changes to this
> section should be updated before merging to reflect any discussion that took
> place on the PR that made those changes.



### INITIAL NOTES

* What do we store in the block?
	* Just bytes are stored in the block at the moment:
	https://github.com/tendermint/tendermint/blob/2b94a0be8273e5f1af2b0b9303eff598adab9c2b/proto/tendermint/types/types.proto
	* These appear to be the bytes of the original Tx as broadcast by the user
		* double check this assumption
* What do clients query for?
	*  https://github.com/tendermint/tendermint/blob/2b94a0be8273e5f1af2b0b9303eff598adab9c2b/types/mempool.go
	*  sha256 hashes are searched for by clients when removing txs
		* Where is this hashing done?
			https://github.com/tendermint/tendermint/blob/2b94a0be8273e5f1af2b0b9303eff598adab9c2b/internal/mempool/tx.go
			* stored in the mempool as sha256 hash
	*  https://github.com/tendermint/tendermint/blob/2b94a0be8273e5f1af2b0b9303eff598adab9c2b/light/rpc/client.go
	* Clients add Txs as a blob of bytes when broadcasting the transaction
	* https://github.com/tendermint/tendermint/blob/2b94a0be8273e5f1af2b0b9303eff598adab9c2b/types/events.go
	* Search by hash when using the `tx_search` endpoint
	* At the moment, there is a 'hash' and 'key' method, both of which do the same thing, but implemented in different libraries
* What would implementing this require?
	* From Tendermint Consensus
		* Mempool would need to be told what to remove:
			* Removed Txs are gossiped via some 2ndary channel
			* Application provides deterministic way to map Txs
			* Alternatively, blocks would contain the set of Hashes for replaced Txs
		* Mechanism for clients to determine if their Tx was executed
			* Txs may be removed from the block.
			* How can clients know if their Tx was executed?
	* From Applications
		* Either: we provide a mechanism for applications to gossip what was replaced
		* This could be within the block or as part of a secondary mechanism
			* Proofs are built off of hashes at the moment
				https://github.com/tendermint/tendermint/blob/2b94a0be8273e5f1af2b0b9303eff598adab9c2b/types/tx.go
	* From Tendermint p2p
		* Either add a secondary channel to broadcast the tx
		* add the Tx to the block structure
			* This breaks the block structure anyway
* Why might we want to do this?
	* Why might my applications want this?
		* Applications may want to recombine a set of txs.
		* Is there literally any advantage to this over just executing all of Txs?
* What kind of bad behavior does this allow?
	* Can you verify that a proposer actually did the substitution correctly?
		* what stops a malicious proposer from just selecting Txs is doesn't want
		* executed as the substitution?
	* Probably not! 
