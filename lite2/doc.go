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
*/
package lite
