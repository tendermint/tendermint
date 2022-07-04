---
order: 1
parent:
    title: blocksync
    order: 3
---

# Blocksync

This section describes the blocksync service provided by Tendermint. The main functionality provided by it is enabling new or recovering nodes to sync up to the head of the chain quickly. Normally, in a proof-of-stake blockchain, blocks are decided when multiple nodes reach consensus on it, requiring many rounds of communication. Once a block is decided on, it is executed against the application and stored, together with the proof that at least 2/3 of the nodes have voted for it. This proof takes the shape of a `Commit` which stores the signatures of 2/3+ validators that approved the block.

 It is sufficient for a syncing node to contact its peers, download old blocks in parallel, verify their commit signatures and execute them sequentially (in order) against the application. As the signatures that validated a block at height `H` are stored with the block at height `H + 1`, blocksync needs to download both blocks in order to verify the former.

We can think about the blocksync service having two modes of operation. One is as a *server*, providing syncing nodes with blocks and commits, the other as a *client* requesting blocks from peers. Each full node runs a blocksync reactor, which launches both the server and client routines. 

Blocksync is implemented within a Reactor whose internal data structures keep track of peers connected to a node and keep a pool of blocks downloaded from them if the node is currently syncing. Blocks are applied one by one until a node is considered caught up (more details on the conditions can be found in the sections below). 

Once caught up, the node switches to consensus and stops running blocksync as a client.

### Starting Blocksync

A node can switch to blocksync directly on start-up or after completing `state-sync`. [State sync](ToDo) is downloading only snapshots of application state along with commits. This is further speeding up the process of catching up to the latest height as a node does not download the actual blocks.

The blocksync reactor service is started at the same time as all the other services in Tendermint. But blocksync-inc is disabled (blockSync boolean flag is false) initially and thus the blockpool and the routine to process blocks from the pool are not launched until the reactor is actually activated. 

The reactor is activated after state sync, where the pool and request processing routines are launched. 

However, receiving messages via the p2p channel and sending status updates to other nodes is enabled regardless of whether the blocksync reactor is started. Essentially every node is running as a blocksync server as soon as it is started up.

**Note**. In the current version, if we start from state sync and blocksync is not launched before as a service, the internal channels used by the reactor will not be created. We need to be careful to launch the blocksync *service* before we call the function to switch from statesync to blocksync.  

### Switching from blocksync to consensus

Ideally, the switch to consensus is done once the node considers itself caugh up or we has not advanced its height for more than a predefined amount of time (currently 60s). 

We consider a node to be caught up if it is 1 height away from the maximum height reported by its peers. The reason we **do not catch up until the maximum height** (`pool.maxPeerHeight`) is that we cannot verify the block at `pool.maxPeerHeight` without the `lastCommit` of the block at `pool.maxPeerHeight + 1`. 

This check is performed periodically by calling `isCaughtUp` inside `poolRoutine`. 

When the node is starting from genesis, the first block does not need the vote extensions and is able to switch directly to consensus.


**Note** It is currently assumed that the Consensus reactor is already running. It is therefore not turned on by the Blocksync reactor. In case it has not been started, the Blocksync reactor simply returns.

#### **Vote extensions in v.036**
In v0.36, Tendermint introduced vote extensions, which is application defined data, attached to the votes deciding on a block. In order to actively participate in consensus, a node has to have vote extensions from the previous height. Therefore, in addition to storing and downloading commits, nodes download and send vote extensions to their peers as well.     

For this reason, if the node is not starting from genesis but only after state sync, blocksync **does not** switch to consensus until it syncs at least one block. As state sync only stores a state snapshot (without vote extensions), we need to receive them from one of our peers.

### Switching to blocksync from consensus

Ideally, a node should be able to switch to blocksync even if it does not crash, but falls behind in consensus. Unfortunately, switching back to blocksync from consensus is not possible at the moment. It is expected to be handled in [Issue #129](https://github.com/tendermint/tendermint/issues/129).

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


