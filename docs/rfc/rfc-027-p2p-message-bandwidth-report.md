# RFC 27: P2P Message Bandwidth Report

## Changelog

- Nov 7, 2022: initial draft (@williambanfield)

## Abstract

Node operators and application developers complain that Tendermint nodes consume
larges amounts of network bandwidth. This RFC catalogues the major sources of bandwidth
consumption within Tendermint and suggests modifications to Tendermint that may reduce
bandwidth consumption for nodes.

## Background

* Osmosis Bandwidth Usage Complaint. How much is this?
* This has high cost for node operators. Find gcloud bandwidth costs. https://cloud.google.com/vpc/network-pricing, hetzner: https://docs.hetzner.com/robot/general/traffic/, find AWS.
* 

## Discussion

### List of top Bandwidth Users in testnet

setup:
https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/docs/qa/method.md#L46
2 conn, 100 tx / s

using v0 mempool
not performing blocksync or statesync.

1. [consensus.BlockPart](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/proto/tendermint/consensus/types.proto#L44)
2. [mempool.Txs](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/proto/tendermint/mempool/types.proto#L6)
3. [consensus.Vote](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/proto/tendermint/consensus/types.proto#L51)

### What are the areas it uses lots of bandwidth, what evidence do we have for this, and what possible fixes exist?

### Overview of Message Usage

#### BlockPart Transmission

Tendermint starts a new `gossipDataRoutine` for each peer it connects to.
Pick a new part of the block that we do not think the peer knows about yet: https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L551 and gossip it to the peer.
We update the set of parts we think it doesn't know about in one of a few ways:
1. We receive a block part from the peer: https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L324.
2. We send the peer a block part: https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L566, https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L684.
3. Our peer tells us about the parts they have block via `NewValidBlock` messages, https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L268. They only send this message when the have all prevotes necessary or are about to commit (all precommits necessary).
4. Our peer tells us they have entered a new round via a `NewRoundStep` message, https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L266

Our peers inform us of their block parts by sending a `NewValidBlockMessage` in [broadcastNewValidBlockMessage](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L446). This only occurs when the block is
becoming the valid value, i.e., it has received 2/3+ prevotes in a round. It also must be 'completed',
otherwise the `cs.ProposalBlock` field would not be set and therefore the [HashesTo](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/state.go#L2120) method on that struct would return false, preventing the code from proceeding to the invocation of the valid block message.

It also occurs when we are about to commit. At this point, we must have the block as we are about to execute it.

uses:
1. https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L560
2. https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L678

#### Mempool Tx Transmission
1. https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/mempool/v0/reactor.go#L247

Tendermint starts a new [broadcastTxRoutine](https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/mempool/v0/reactor.go#L197) for each peer that it is informed of. The routine sends all Tx the mempool is aware of to all peers. If the mempool has received a tx from a peer, then it marks it as such and won't resend. Otherwise, it retains no information about which transactions the peer has received. It may therefore resend transactions the peer already has. Additionally, if the head of the clist is deleted while the mempool loop is examining it, it will begin at the beginning of the list, re-sending all previously sent transactions to the peer.


#### Vote Transmission
1. https://github.com/tendermint/tendermint/blob/ff0f98892f24aac11e46aeff2b6d2c0ad816701a/consensus/reactor.go#L1132

### Shortcomings of current mechanisms

## References
