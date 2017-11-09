/*
Package lite allows you to securely validate headers
without a full node.

This library pulls together all the crypto and algorithms,
so given a relatively recent (< unbonding period) known
validator set, one can get indisputable proof that data is in
the chain (current state) or detect if the node is lying to
the client.

Tendermint RPC exposes a lot of info, but a malicious node
could return any data it wants to queries, or even to block
headers, even making up fake signatures from non-existent
validators to justify it. This is a lot of logic to get
right, to be contained in a small, easy to use library,
that does this for you, so you can just build nice UI.

We design for clients who have no strong trust relationship
with any tendermint node, just the validator set as a whole.
Beyond building nice mobile or desktop applications, the
cosmos hub is another important example of a client,
that needs undeniable proof without syncing the full chain,
in order to efficiently implement IBC.

Commits

There are two main data structures that we pass around - Commit
and FullCommit. Both of them mirror what information is
exposed in tendermint rpc.

Commit is a block header along with enough validator signatures
to prove its validity (> 2/3 of the voting power). A FullCommit
is a Commit along with the full validator set. When the
validator set doesn't change, the Commit is enough, but since
the block header only has a hash, we need the FullCommit to
follow any changes to the validator set.

Certifiers

A Certifier validates a new Commit given the currently known
state. There are three different types of Certifiers exposed,
each one building on the last one, with additional complexity.

Static - given the validator set upon initialization. Verifies
all signatures against that set and if the validator set
changes, it will reject all headers.

Dynamic - This wraps Static and has the same Certify
method. However, it adds an Update method, which can be called
with a FullCommit when the validator set changes. If it can
prove this is a valid transition, it will update the validator
set.

Inquiring - this wraps Dynamic and implements an auto-update
strategy on top of the Dynamic update. If a call to
Certify fails as the validator set has changed, then it
attempts to find a FullCommit and Update to that header.
To get these FullCommits, it makes use of a Provider.

Providers

A Provider allows us to store and retrieve the FullCommits,
to provide memory to the Inquiring Certifier.

NewMemStoreProvider - in-memory cache.

files.NewProvider - disk backed storage.

client.NewHTTPProvider - query tendermint rpc.

NewCacheProvider - combine multiple providers.

The suggested use for local light clients is
client.NewHTTPProvider for getting new data (Source),
and NewCacheProvider(NewMemStoreProvider(),
files.NewProvider()) to store confirmed headers (Trusted)

How We Track Validators

Unless you want to blindly trust the node you talk with, you
need to trace every response back to a hash in a block header
and validate the commit signatures of that block header match
the proper validator set.  If there is a contant validator
set, you store it locally upon initialization of the client,
and check against that every time.

Once there is a dynamic validator set, the issue of
verifying a block becomes a bit more tricky. There is
background information in a
github issue (https://github.com/tendermint/tendermint/issues/377).

In short, if there is a block at height H with a known
(trusted) validator set V, and another block at height H'
(H' > H) with validator set V' != V, then we want a way to
safely update it.

First, get the new (unconfirmed) validator set V' and
verify H' is internally consistent and properly signed by
this V'. Assuming it is a valid block, we check that at
least 2/3 of the validators in V also signed it, meaning
it would also be valid under our old assumptions.
That should be enough, but we can also check that the
V counts for at least 2/3 of the total votes in H'
for extra safety (we can have a discussion if this is
strictly required). If we can verify all this,
then we can accept H' and V' as valid and use that to
validate all blocks X > H'.

If we cannot update directly from H -> H' because there was
too much change to the validator set, then we can look for
some Hm (H < Hm < H') with a validator set Vm.  Then we try
to update H -> Hm and Hm -> H' in two separate steps.
If one of these steps doesn't work, then we continue
bisecting, until we eventually have to externally
validate the valdiator set changes at every block.

Since we never trust any server in this protocol, only the
signatures themselves, it doesn't matter if the seed comes
from a (possibly malicious) node or a (possibly malicious) user.
We can accept it or reject it based only on our trusted
validator set and cryptographic proofs. This makes it
extremely important to verify that you have the proper
validator set when initializing the client, as that is the
root of all trust.

Or course, this assumes that the known block is within the
unbonding period to avoid the "nothing at stake" problem.
If you haven't seen the state in a few months, you will need
to manually verify the new validator set hash using off-chain
means (the same as getting the initial hash).

*/
package lite
