/*
Package blocksync implements two versions of a reactor Service that are
responsible for block propagation and gossip between peers. This mechanism was
formerly known as fast-sync.

In order for a full node to successfully participate in consensus, it must have
the latest view of state. The blocksync protocol is a mechanism in which peers
may exchange and gossip entire blocks with one another, in a request/response
type model, until they've successfully synced to the latest head block. Once
succussfully synced, the full node can switch to an active role in consensus and
will no longer blocksync and thus no longer run the blocksync process.

Note, the blocksync reactor Service gossips entire block and relevant data such
that each receiving peer may construct the entire view of the blocksync state.

There is currently only one version of the blocksync reactor Service
that is battle-tested, but whose test coverage is lacking and is not
formally verified.

The v0 blocksync reactor Service has one p2p channel, BlockchainChannel. This
channel is responsible for handling messages that both request blocks and respond
to block requests from peers. For every block request from a peer, the reactor
will execute respondToPeer which will fetch the block from the node's state store
and respond to the peer. For every block response, the node will add the block
to its pool via AddBlock.

Internally, v0 runs a poolRoutine that constantly checks for what blocks it needs
and requests them. The poolRoutine is also responsible for taking blocks from the
pool, saving and executing each block.
*/
package blocksync
