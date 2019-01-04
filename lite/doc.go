/*
Package lite allows you to securely validate headers without a full node.

This library pulls together all the crypto and algorithms, so given a
relatively recent (< unbonding period) known validator set, one can get
indisputable proof that data is in the chain (current state) or detect if the
node is lying to the client.

Tendermint RPC exposes a lot of info, but a malicious node could return any
data it wants to queries, or even to block headers, even making up fake
signatures from non-existent validators to justify it. This is a lot of logic
to get right, to be contained in a small, easy to use library, that does this
for you, so you can just build nice applications.

We design for clients who have no strong trust relationship with any Tendermint
node, just the blockchain and validator set as a whole.

# Data structures

## SignedHeader

SignedHeader is a block header along with a commit -- enough validator
precommit-vote signatures to prove its validity (> 2/3 of the voting power)
given the validator set responsible for signing that header. A FullCommit is a
SignedHeader along with the current and next validator sets.

The hash of the next validator set is included and signed in the SignedHeader.
This lets the lite client keep track of arbitrary changes to the validator set,
as every change to the validator set must be approved by inclusion in the
header and signed in the commit.

In the worst case, with every block changing the validators around completely,
a lite client can sync up with every block header to verify each validator set
change on the chain. In practice, most applications will not have frequent
drastic updates to the validator set, so the logic defined in this package for
lite client syncing is optimized to use intelligent bisection and
block-skipping for efficient sourcing and verification of these data structures
and updates to the validator set (see the DynamicVerifier for more
information).

The FullCommit is also declared in this package as a convenience structure,
which includes the SignedHeader along with the full current and next
ValidatorSets.

## Verifier

A Verifier validates a new SignedHeader given the currently known state. There
are two different types of Verifiers provided.

BaseVerifier - given a validator set and a height, this Verifier verifies
that > 2/3 of the voting power of the given validator set had signed the
SignedHeader, and that the SignedHeader was to be signed by the exact given
validator set, and that the height of the commit is at least height (or
greater).

SignedHeader.Commit may be signed by a different validator set, it can get
verified with a BaseVerifier as long as sufficient signatures from the
previous validator set are present in the commit.

DynamicVerifier - this Verifier implements an auto-update and persistence
strategy to verify any SignedHeader of the blockchain.

## Provider and PersistentProvider

A Provider allows us to store and retrieve the FullCommits.

```go
type Provider interface {
	// LatestFullCommit returns the latest commit with
	// minHeight <= height <= maxHeight.
	// If maxHeight is zero, returns the latest where
	// minHeight <= height.
	LatestFullCommit(chainID string, minHeight, maxHeight int64) (FullCommit, error)
}
```

* client.NewHTTPProvider - query Tendermint rpc.

A PersistentProvider is a Provider that also allows for saving state.  This is
used by the DynamicVerifier for persistence.

```go
type PersistentProvider interface {
	Provider

	// SaveFullCommit saves a FullCommit (without verification).
	SaveFullCommit(fc FullCommit) error
}
```

* DBProvider - persistence provider for use with any libs/DB.
* MultiProvider - combine multiple providers.

The suggested use for local light clients is client.NewHTTPProvider(...) for
getting new data (Source), and NewMultiProvider(NewDBProvider("label",
dbm.NewMemDB()), NewDBProvider("label", db.NewFileDB(...))) to store confirmed
full commits (Trusted)


# How We Track Validators

Unless you want to blindly trust the node you talk with, you need to trace
every response back to a hash in a block header and validate the commit
signatures of that block header match the proper validator set.  If there is a
static validator set, you store it locally upon initialization of the client,
and check against that every time.

If the validator set for the blockchain is dynamic, verifying block commits is
a bit more involved -- if there is a block at height H with a known (trusted)
validator set V, and another block at height H' (H' > H) with validator set V'
!= V, then we want a way to safely update it.

First, we get the new (unconfirmed) validator set V' and verify that H' is
internally consistent and properly signed by this V'. Assuming it is a valid
block, we check that at least 2/3 of the validators in V also signed it,
meaning it would also be valid under our old assumptions.  Then, we accept H'
and V' as valid and trusted and use that to validate for heights X > H' until a
more recent and updated validator set is found.

If we cannot update directly from H -> H' because there was too much change to
the validator set, then we can look for some Hm (H < Hm < H') with a validator
set Vm.  Then we try to update H -> Hm and then Hm -> H' in two steps.  If one
of these steps doesn't work, then we continue bisecting, until we eventually
have to externally validate the validator set changes at every block.

Since we never trust any server in this protocol, only the signatures
themselves, it doesn't matter if the seed comes from a (possibly malicious)
node or a (possibly malicious) user.  We can accept it or reject it based only
on our trusted validator set and cryptographic proofs. This makes it extremely
important to verify that you have the proper validator set when initializing
the client, as that is the root of all trust.

The software currently assumes that the unbonding period is infinite in
duration.  If the DynamicVerifier hasn't been updated in a while, you should
manually verify the block headers using other sources.

TODO: Update the software to handle cases around the unbonding period.

*/
package lite
