/*
Package blockchain implements two versions of a reactor Service that are
responsible for block propagation and gossip between peers. This mechanism is
more formally known as fast-sync.

In order for a validator to successfully participate in consensus, it must have
the latest view of state. The fast-sync protocol is a mechanism in which peers
may exchange and gossip entire blocks with one another, in a request/response
type model, until they've successfully synced to the latest head block. Once
succussfully synced, the validator can switch to an active role in consensus and
will no longer fast-sync and thus no longer run the fast-sync process.

Note, the blockchain reactor Service gossips entire block and relevant data such
that each receiving peer may construct the entire view of the blockchain state.

There are two versions of the blockchain reactor Service, i.e. fast-sync:

- v0: The initial implementation that is battle-tested, but whose test coverage
	is lacking and is not formally verifiable.
- v2: The latest implementation that has much higher test coverage and is formally
	verified. However, the current implementation of v2 is not as battle-tested and
	is known to have various bugs that could make it unreliable in production
	environments.

The v0 blockchain reactor Service has one p2p channel, BlockchainChannel. This
channel is responsible for handling messages that both request blocks and respond
to block requests from peers. For every block request from a peer, the reactor
will execute respondToPeer which will fetch the block from the node's state store
and respond to the peer. For every block response, the node will add the block
the it's pool via AddBlock.

Internally, v0 runs a poolRoutine that constantly checks for what blocks it needs
and requests them. The poolRoutine is also responsible for taking blocks from the
pool, saving and executing each block.
*/
package blockchain
