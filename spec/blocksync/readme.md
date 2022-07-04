---
order: 1
parent:
    title: blocksync
    order: 3
---

# Blocksync

In a proof of work blockchain, syncing with the chain is the same process as staying up-to-date with the consensus: download blocks, and look for the one with the most total work. In proof-of-stake, the consensus process is more complex, as it involves rounds of communication between the nodes to determine what block should be committed next. Using this process to sync up with the blockchain from scratch can take a very long time. It's much faster to just download blocks and check the merkle tree of validators than to run the real-time consensus gossip protocol.

The Blocksync Reactor's high level responsibility is to enable peers who are
far behind the current state of the consensus to quickly catch up by downloading
many blocks (that have already been decided) in parallel, verifying their commits, and executing them against the
ABCI application.

Tendermint full nodes run the Blocksync Reactor as a service to provide blocks
to new or recovering nodes. The nodes run the Blocksync Reactor in "fast_sync" mode,
where they actively make requests for more blocks until they sync up.
Once caught up, "fast_sync" mode is disabled and the node switches to
using the Consensus Reactor. 

*Note* It is currently assumed that the Consensus reactor is already running. It is therefore not turned on by the Blocksync reactor. In case it has not been started, the Blocksync reactor simply returns.

### Conditions to start Blocksync

A node can switch to blocksync directly on start-up or after completing `state-sync`. Currently, switching back to blocksync from consensus is not possible. It is expected to be handled in [Issue #129](https://github.com/tendermint/tendermint/issues/129).

The blocksync reactor service is started at the same time as all the other services in Tendermint. But blocksync-inc is disabled (blockSync boolean flag is false) initially and thus the blockpool and the routine to process blocks from the pool are not launched until the reactor is actually activated. 

The reactor is activated after state sync, where the pool and request processing routines are launched. 

However, receiving messages via the p2p channel and sending status updates to other nodes is enabled regardless of whether the blocksync reactor is started. This makes sense as a node should be able to send updates to other peers regardless of whether it itself is blocksyncing.  

**Note**. In the current version, if we start from state sync and block sync is not launched before as a service, the internal channels used by the reactor will not be created. We need to be careful to launch the blocksync *service* before we call the function to switch from statesync to blocksync.  

### Switching from blocksync to consensus
Ideally, the switch to consensus is done once the node considers itself caugh up or we have not advanced our height for more than 60s. 

The former is checked by calling `isCaughtUp` inside `poolRoutine` periodically. This period is set with `switchToConsensusTicker`. We consider a node to be caught up if it is 1 height away from the maximum height reported by its peers. The reason we **do not catch up until the maximum height** (`pool.maxPeerHeight`) is that we cannot verify the block at `pool.maxPeerHeight` without the `lastCommit` of the block at `pool.maxPeerHeight + 1`. 

If the node is not starting from genesis, blocksync **does not** switch to consensus until we have synced at least one block. We need to have vote extensions in order to participate in consensus and they are not provided to the blocksync reactor after state sync. We therefore need to receive them from one of our peers. 

When the node is starting from genesis, the first block does not need the vote extensions and is able to switch directly to consensus.

## Architecture and algorithm

The Blocksync reactor is organised as a set of concurrent tasks:

- Receive routine of Blocksync Reactor
- Task for creating Requesters
- Set of Requester tasks 
- A controller task.


![Blocksync Reactor Architecture Diagram](img/bc-reactor.png)

This section describes the Blocksync reactor and its internals including:
- [Data structures used](./data_structures.md)
- [Peer to peer communication pattern](./communication.md)
- [Block Verification](./verification.md)

More details on how to use the Blocksync reactor and configure it when running Tendermint can be found [here](./../../docs/tendermint-core/block-sync/README.md).


