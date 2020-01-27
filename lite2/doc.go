/*
Package lite provides a light client implementation.

The concept of light clients was introduced in the Bitcoin white paper. It
describes a watcher of distributed consensus process that only validates the
consensus algorithm and not the state machine transactions within.

Tendermint light clients allow bandwidth & compute-constrained devices, such as
smartphones, low-power embedded chips, or other blockchains to efficiently
verify the consensus of a Tendermint blockchain. This forms the basis of safe
and efficient state synchronization for new network nodes and inter-blockchain
communication (where a light client of one Tendermint instance runs in another
chain's state machine).

In a network that is expected to reliably punish validators for misbehavior by
slashing bonded stake and where the validator set changes infrequently, clients
can take advantage of this assumption to safely synchronize a lite client
without downloading the intervening headers.

Light clients (and full nodes) operating in the Proof Of Stake context need a
trusted block height from a trusted source that is no older than 1 unbonding
window plus a configurable evidence submission synchrony bound. This is called
weak subjectivity.

Weak subjectivity is required in Proof of Stake blockchains because it is
costless for an attacker to buy up voting keys that are no longer bonded and
fork the network at some point in its prior history. See Vitalik's post at
[Proof of Stake: How I Learned to Love Weak
Subjectivity](https://blog.ethereum.org/2014/11/25/proof-stake-learned-love-weak-subjectivity/).

NOTE: Tendermint provides a somewhat different (stronger) light client model
than Bitcoin under eclipse, since the eclipsing node(s) can only fool the light
client if they have two-thirds of the private keys from the last root-of-trust.

# Common structures

* SignedHeader

SignedHeader is a block header along with a commit -- enough validator
precommit-vote signatures to prove its validity (> 2/3 of the voting power)
given the validator set responsible for signing that header.

The hash of the next validator set is included and signed in the SignedHeader.
This lets the lite client keep track of arbitrary changes to the validator set,
as every change to the validator set must be approved by inclusion in the
header and signed in the commit.

In the worst case, with every block changing the validators around completely,
a lite client can sync up with every block header to verify each validator set
change on the chain. In practice, most applications will not have frequent
drastic updates to the validator set, so the logic defined in this package for
lite client syncing is optimized to use intelligent bisection.

# What this package provides

This package provides three major things:

1. Client implementation (see client.go)
2. Pure functions to verify a new header (see verifier.go)
3. Secure RPC proxy

## 1. Client implementation (see client.go)

Example usage:

		db, err := dbm.NewGoLevelDB("lite-client-db", dbDir)
		if err != nil {
			// return err
			t.Fatal(err)
		}
		c, err := NewClient(
			chainID,
			TrustOptions{
				Period: 504 * time.Hour, // 21 days
				Height: 100,
				Hash:   header.Hash(),
			},
			httpp.New(chainID, "tcp://localhost:26657"),
			dbs.New(db, chainID),
		)

		err = c.VerifyHeaderAtHeight(101, time.Now())
		if err != nil {
			fmt.Println("retry?")
		}

		h, err := c.TrustedHeader(101)
		if err != nil {
			fmt.Println("retry?")
		}
		fmt.Println("got header", h)

## 2. Pure functions to verify a new header (see verifier.go)

Verify function verifies a new header against some trusted header. See
https://github.com/tendermint/spec/blob/master/spec/consensus/light-client.md
for details.

## 3. Secure RPC proxy

Tendermint RPC exposes a lot of info, but a malicious node could return any
data it wants to queries, or even to block headers, even making up fake
signatures from non-existent validators to justify it. Secure RPC proxy serves
as a wrapper, which verifies all the headers, using a light client connected to
some other node.

See
https://github.com/tendermint/tendermint/blob/master/cmd/tendermint/commands/lite.go
for usage example.
*/
package lite
