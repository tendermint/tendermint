----------------------------- MODULE fastsync -----------------------------
(*
 In this document we give the high level specification of the fast sync
 protocol as implemented here:
 https://github.com/tendermint/tendermint/tree/master/blockchain/v2.

We assume a system in which one node is trying to sync with the blockchain
(replicated state machine) by downloading blocks from the set of full nodes
(we call them peers) that are block providers, and executing transactions
(part of the block) against the application.

Peers can be faulty, and we don't make any assumption about the rate of correct/faulty
nodes in the node peerset (i.e., they can all be faulty). Correct peers are part
of the replicated state machine, i.e., they manage blockchain and execute
transactions against the same deterministic application. We don't make any
assumptions about the behavior of faulty processes. Processes (client and peers)
communicate by message passing.

 In this specification, we model this system with two parties:
    - the node (state machine) that is doing fastsync and
    - the environment with which node interacts.

The environment consists of the set of (correct and faulty) peers with
which node interacts as part of fast sync protocol, but also contains some
aspects (adding/removing peers, timeout mechanism) that are part of the node
local environment (could be seen as part of the runtime in which node
executes).

As part of the fast sync protocol a node and the peers exchange the following messages:

- StatusRequest
- StatusResponse
- BlockRequest
- BlockResponse
- NoBlockResponse.

A node is periodically issuing StatusRequests to query peers for their current height (to decide what
blocks to ask from what peers). Based on StatusResponses (that are sent by peers), the node queries
blocks for some height(s) by sending peers BlockRequest messages. A peer provides a requested block by
BlockResponse message. If a peer does not want to provide a requested block, then it sends NoBlockResponse message.
In addition to those messages, a node in this spec receives additional input messages (events):

- AddPeer
- RemovePeer
- SyncTimeout.

These are the control messages that are provided to the node by its execution enviornment. AddPeer
is for the case when a connection is established with a peer; similarly RemovePeer is for the case
a connection with the peer is terminated. Finally SyncTimeout is used to model a timeout trigger.

We assume that fast sync protocol starts when connections with some number of peers
are established. Therefore, peer set is initialised with non-empty set of peer ids. Note however
that node does not know initially the peer heights.
*)
 
EXTENDS Integers, FiniteSets, Sequences


CONSTANTS MAX_HEIGHT,                \* the maximal height of blockchain
          VALIDATOR_SETS,            \* abstract set of validators
          NIL_VS,                    \* a nil validator set
          CORRECT,                   \* set of correct peers
          FAULTY,                    \* set of faulty peers
          TARGET_PENDING,            \* maximum number of pending requests + downloaded blocks that are not yet processed
          PEER_MAX_REQUESTS          \* maximum number of pending requests per peer

ASSUME CORRECT \intersect FAULTY = {}
ASSUME TARGET_PENDING > 0
ASSUME PEER_MAX_REQUESTS > 0

\* the blockchain, see Tinychain
VARIABLE chain
  
\* introduce tiny chain as the source of blocks for the correct nodes
INSTANCE Tinychain

\* a special value for an undefined height
NilHeight == 0

\* the height of the genesis block
TrustedHeight == 1

\* the set of all peer ids the node can receive a message from
AllPeerIds == CORRECT \union FAULTY

\* Correct last commit have enough voting power, i.e., +2/3 of the voting power of
\* the corresponding validator set (enoughVotingPower = TRUE) that signs blockId.
\* BlockId defines correct previous block (in the implementation it is the hash of the block).
\* Instead of blockId, we encode blockIdEqRef, which is true, if the block id is equal
\* to the hash of the previous block, see Tinychain.
CorrectLastCommit(h) == chain[h].lastCommit

NilCommit == [blockIdEqRef |-> FALSE, committers |-> NIL_VS]

\* correct node always supplies the blocks from the blockchain
CorrectBlock(h) == chain[h]

NilBlock ==
    [height |-> 0, hashEqRef |-> FALSE, wellFormed |-> FALSE,
     lastCommit |-> NilCommit, VS |-> NIL_VS, NextVS |-> NIL_VS]

\* a special value for an undefined peer
NilPeer == "Nil" \* STRING for apalache efficiency

\* control the state of the syncing node
States == { "running", "finished"}

NoMsg == [type |-> "None"]

\* the variables of the node running fastsync
VARIABLES
  state,                                     \* running or finished
  (*
  blockPool [
    height,                                 \* current height we are trying to sync. Last block executed is height - 1
    peerIds,                                \* set of peers node is connected to
    peerHeights,                            \* map of peer ids to its (stated) height
    blockStore,                             \* map of heights to (received) blocks
    receivedBlocks,                         \* map of heights to peer that has sent us the block (stored in blockStore)
    pendingBlocks,                          \* map of heights to peer to which block request has been sent
    syncHeight,                             \* height at the point syncTimeout was triggered last time
    syncedBlocks                            \* number of blocks synced since last syncTimeout. If it is 0 when the next timeout occurs, then protocol terminates.
  ]
  *)
  blockPool
  

\* the variables of the peers providing blocks
VARIABLES
  (*
  peersState [
    peerHeights,                             \* track peer heights
    statusRequested,                         \* boolean set to true when StatusRequest is received. Models periodic sending of StatusRequests.
    blocksRequested                          \* set of BlockRequests received that are not answered yet
  ]
  *)
  peersState

 \* the variables for the network and scheduler
VARIABLES
  turn,                                     \* who is taking the turn: "Peers" or "Node"
  inMsg,                                    \* a node receives message by this variable
  outMsg                                    \* a node sends a message by this variable


(* the variables of the node *)
nvars == <<state, blockPool>>

(*************** Type definitions for Apalache (model checker) **********************)
AsIntSet(S) == S <: {Int}

\* type of process ids
PIDT == STRING
AsPidSet(S) == S <: {PIDT}

\* ControlMessage type
CMT == [type |-> STRING, peerId |-> PIDT] \* type of control messages

\* InMsg type
IMT == [type |-> STRING, peerId |-> PIDT, height |-> Int, block |-> BT]
AsInMsg(m) == m <: IMT
AsInMsgSet(S) == S <: {IMT}

\* OutMsg type
OMT == [type |-> STRING, peerId |-> PIDT, height |-> Int]
AsOutMsg(m) == m <: OMT
AsOutMsgSet(S) == S <: {OMT}

\* block pool type
BPT == [height |-> Int, peerIds |-> {PIDT}, peerHeights |-> [PIDT -> Int],
        blockStore |-> [Int -> BT], receivedBlocks |-> [Int -> PIDT],
        pendingBlocks |-> [Int -> PIDT], syncedBlocks |-> Int, syncHeight |-> Int]

AsBlockPool(bp) == bp <: BPT

(******************** Sets of messages ********************************)

\* Control messages
ControlMsgs ==
    AsInMsgSet([type: {"addPeer"}, peerId: AllPeerIds])
        \union
    AsInMsgSet([type: {"removePeer"}, peerId: AllPeerIds])
        \union
    AsInMsgSet([type: {"syncTimeout"}])

\* All messages (and events) received by a node
InMsgs ==
    AsInMsgSet({NoMsg})
        \union
    AsInMsgSet([type: {"blockResponse"}, peerId: AllPeerIds, block: Blocks])
        \union
    AsInMsgSet([type: {"noBlockResponse"}, peerId: AllPeerIds, height: Heights])
        \union    
    AsInMsgSet([type: {"statusResponse"}, peerId: AllPeerIds, height: Heights])
        \union
    ControlMsgs

\* Messages sent by a node and received by peers (environment in our case)
OutMsgs ==
    AsOutMsgSet({NoMsg})
        \union
    AsOutMsgSet([type: {"statusRequest"}]) \* StatusRequest is broadcast to the set of connected peers.
        \union
    AsOutMsgSet([type: {"blockRequest"}, peerId: AllPeerIds, height: Heights])


(********************************** NODE ***********************************)

InitNode ==
     \E pIds \in SUBSET AllPeerIds:                   \* set of peers node established initial connections with
        /\ pIds \subseteq CORRECT   \* this line is not necessary
        /\ pIds /= AsPidSet({}) \* apalache better checks non-emptiness than subtracts from SUBSET
        /\ blockPool = AsBlockPool([
                height |-> TrustedHeight + 1,       \* the genesis block is at height 1
                syncHeight |-> TrustedHeight + 1,   \* and we are synchronized to it
                peerIds |-> pIds,
                peerHeights |-> [p \in AllPeerIds |-> NilHeight],     \* no peer height is known
                blockStore |->
                    [h \in Heights |->
                      IF h > TrustedHeight THEN NilBlock ELSE chain[1]],
                receivedBlocks |-> [h \in Heights |-> NilPeer],
                pendingBlocks |-> [h \in Heights |-> NilPeer],
                syncedBlocks |-> -1
           ])
       /\ state = "running"

\* Remove faulty peers.
\* Returns new block pool.
\* See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L222
RemovePeers(rmPeers, bPool) ==
    LET keepPeers == bPool.peerIds \ rmPeers IN
    LET pHeights ==
        [p \in AllPeerIds |-> IF p \in rmPeers THEN NilHeight ELSE bPool.peerHeights[p]] IN

    LET failedRequests ==
        {h \in Heights: /\ h >= bPool.height
                        /\ \/ bPool.pendingBlocks[h] \in rmPeers
                           \/ bPool.receivedBlocks[h] \in rmPeers} IN
    LET pBlocks ==
        [h \in Heights |-> IF h \in failedRequests THEN NilPeer ELSE bPool.pendingBlocks[h]] IN
    LET rBlocks ==
        [h \in Heights |-> IF h \in failedRequests THEN NilPeer ELSE bPool.receivedBlocks[h]] IN
    LET bStore ==
        [h \in Heights |-> IF h \in failedRequests THEN NilBlock ELSE bPool.blockStore[h]] IN

    IF keepPeers /= bPool.peerIds
    THEN [bPool EXCEPT
            !.peerIds = keepPeers,
            !.peerHeights = pHeights,
            !.pendingBlocks = pBlocks,
            !.receivedBlocks = rBlocks,
            !.blockStore = bStore
          ]
    ELSE bPool

\* Add a peer.
\* see https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L198
AddPeer(peer, bPool) ==
    [bPool EXCEPT !.peerIds = bPool.peerIds \union {peer}]


(*
Handle StatusResponse message.
If valid status response, update peerHeights.
If invalid (height is smaller than the current), then remove peer.
Returns new block pool.
See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L667
*)
HandleStatusResponse(msg, bPool) ==
    LET peerHeight == bPool.peerHeights[msg.peerId] IN

    IF /\ msg.peerId \in bPool.peerIds
       /\ msg.height >= peerHeight
    THEN    \* a correct response
        LET pHeights == [bPool.peerHeights EXCEPT ![msg.peerId] = msg.height] IN
        [bPool EXCEPT !.peerHeights = pHeights]
    ELSE RemovePeers({msg.peerId}, bPool)   \* the peer has sent us message with smaller height or peer is not in our peer list


(*
Handle BlockResponse message.
If valid block response, update blockStore, pendingBlocks and receivedBlocks.
If invalid (unsolicited response or malformed block), then remove peer.
Returns new block pool.
See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L522
*)
HandleBlockResponse(msg, bPool) ==
    LET h == msg.block.height IN

    IF /\ msg.peerId \in bPool.peerIds
       /\ bPool.blockStore[h] = NilBlock
       /\ bPool.pendingBlocks[h] = msg.peerId
       /\ msg.block.wellFormed
    THEN
        [bPool EXCEPT
            !.blockStore = [bPool.blockStore EXCEPT ![h] = msg.block],
            !.receivedBlocks = [bPool.receivedBlocks EXCEPT![h] = msg.peerId],
            !.pendingBlocks = [bPool.pendingBlocks EXCEPT![h] = NilPeer]
         ]
    ELSE RemovePeers({msg.peerId}, bPool)
    
 HandleNoBlockResponse(msg, bPool) ==
    RemovePeers({msg.peerId}, bPool)
       

\* Compute max peer height.
\* See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L440
MaxPeerHeight(bPool) ==
    IF bPool.peerIds = AsPidSet({})
    THEN 0 \* no peers, just return 0
    ELSE LET Hts == {bPool.peerHeights[p] : p \in bPool.peerIds} IN
           CHOOSE max \in Hts: \A h \in Hts: h <= max

(* Returns next height for which request should be sent.
   Returns NilHeight in case there is no height for which request can be sent.
   See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L454 *)
FindNextRequestHeight(bPool) ==
    LET S == {i \in Heights:
                /\ i >= bPool.height
                /\ i <= MaxPeerHeight(bPool)
                /\ bPool.blockStore[i] = NilBlock
                /\ bPool.pendingBlocks[i] = NilPeer} IN
    IF S = AsIntSet({})
        THEN NilHeight
    ELSE
        CHOOSE min \in S:  \A h \in S: h >= min

\* Returns number of pending requests for a given peer.
NumOfPendingRequests(bPool, peer) ==
    LET peerPendingRequests ==
        {h \in Heights:
            /\ h >= bPool.height
            /\ bPool.pendingBlocks[h] = peer
        }
    IN
    Cardinality(peerPendingRequests)

(* Returns peer that can serve block for a given height.
   Returns NilPeer in case there are no such peer.
   See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L477 *)
FindPeerToServe(bPool, h) ==
    LET peersThatCanServe == { p \in bPool.peerIds:
                /\ bPool.peerHeights[p] >= h
                /\ NumOfPendingRequests(bPool, p) < PEER_MAX_REQUESTS } IN

    LET pendingBlocks ==
        {i \in Heights:
            /\ i >= bPool.height
            /\ \/ bPool.pendingBlocks[i] /= NilPeer
               \/ bPool.blockStore[i] /= NilBlock
        } IN

    IF \/ peersThatCanServe = AsPidSet({})
       \/ Cardinality(pendingBlocks) >= TARGET_PENDING
    THEN NilPeer
    \* pick a peer that can serve request for height h that has minimum number of pending requests
    ELSE CHOOSE p \in peersThatCanServe: \A q \in peersThatCanServe:
            /\ NumOfPendingRequests(bPool, p) <= NumOfPendingRequests(bPool, q)


\* Make a request for a block (if possible) and return a request message and block poool.
CreateRequest(bPool) ==
    LET nextHeight == FindNextRequestHeight(bPool) IN

    IF nextHeight = NilHeight THEN [msg |-> AsOutMsg(NoMsg), pool |-> bPool]
    ELSE
     LET peer == FindPeerToServe(bPool, nextHeight) IN
     IF peer = NilPeer THEN [msg |-> AsOutMsg(NoMsg), pool |-> bPool]
     ELSE
        LET m == [type |-> "blockRequest", peerId |-> peer, height |-> nextHeight] IN
        LET newPool == [bPool EXCEPT
                          !.pendingBlocks = [bPool.pendingBlocks EXCEPT ![nextHeight] = peer]
                        ] IN
        [msg |-> m, pool |-> newPool]


\* Returns node state, i.e., defines termination condition.
\* See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L432
ComputeNextState(bPool) ==
    IF bPool.syncedBlocks = 0  \* corresponds to the syncTimeout in case no progress has been made for a period of time.
    THEN "finished"
    ELSE IF /\ bPool.height > 1
            /\ bPool.height >= MaxPeerHeight(bPool) \* see https://github.com/tendermint/tendermint/blob/61057a8b0af2beadee106e47c4616b279e83c920/blockchain/v2/scheduler.go#L566
         THEN "finished"
         ELSE "running"

(* Verify if commit is for the given block id and if commit has enough voting power.
   See https://github.com/tendermint/tendermint/blob/61057a8b0af2beadee106e47c4616b279e83c920/blockchain/v2/processor_context.go#L12 *)
VerifyCommit(block, lastCommit) ==
    PossibleCommit(block, lastCommit)

(* Tries to execute next block in the pool, i.e., defines block validation logic.
   Returns new block pool (peers that has send invalid blocks are removed).
   See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/processor.go#L135 *)
ExecuteBlocks(bPool) ==
    LET bStore == bPool.blockStore IN
    LET block0 == bStore[bPool.height - 1] IN
      \* blockPool is initialized with height = TrustedHeight + 1,
      \* so bStore[bPool.height - 1] is well defined
    LET block1 == bStore[bPool.height] IN
    LET block2 == bStore[bPool.height + 1] IN

    IF block1 = NilBlock \/ block2 = NilBlock
    THEN bPool  \* we don't have two next consecutive blocks

    ELSE IF ~IsMatchingValidators(block1, block0.NextVS)
              \* Check that block1.VS = block0.Next.
              \* Otherwise, CorrectBlocksInv fails.
              \* In the implementation NextVS is part of the application state,
              \* so a mismatch can be found without access to block0.NextVS.
         THEN \* the block does not have the expected validator set
              RemovePeers({bPool.receivedBlocks[bPool.height]}, bPool)
         ELSE IF ~VerifyCommit(block1, block2.lastCommit)  
              \* Verify commit of block2 based on block1.
              \* Interestingly, we do not have to call IsMatchingValidators.
              THEN \* remove the peers of block1 and block2, as they are considered faulty
              RemovePeers({bPool.receivedBlocks[bPool.height],
                           bPool.receivedBlocks[bPool.height + 1]},
                          bPool)
              ELSE  \* all good, execute block at position height
                [bPool EXCEPT !.height = bPool.height + 1]


\* Defines logic for pruning peers.
\* See https://github.com/tendermint/tendermint/blob/dac030d6daf4d3e066d84275911128856838af4e/blockchain/v2/scheduler.go#L613
TryPrunePeer(bPool, suspectedSet, isTimedOut) ==
    (* -----------------------------------------------------------------------------------------------------------------------*)
    (* Corresponds to function prunablePeers in scheduler.go file. Note that this function only checks if block has been  *)
    (* received from a peer during peerTimeout period.                                                                        *)
    (* Note that in case no request has been scheduled to a correct peer, or a request has been scheduled                     *)
    (* recently, so the peer hasn't responded yet, a peer will be removed as no block is received within peerTimeout.         *)
    (* In case of faulty peers, we don't have any guarantee that they will respond.                                           *)
    (* Therefore, we model this with nondeterministic behavior as it could lead to peer removal, for both correct and faulty. *)
    (* See scheduler.go                                                                                                       *)
    (* https://github.com/tendermint/tendermint/blob/4298bbcc4e25be78e3c4f21979d6aa01aede6e87/blockchain/v2/scheduler.go#L335 *)
    LET toRemovePeers == bPool.peerIds \intersect suspectedSet IN

    (*
      Corresponds to logic for pruning a peer that is responsible for delivering block for the next height.
      The pruning logic for the next height is based on the time when a BlockRequest is sent. Therefore, if a request is sent 
      to a correct peer for the next height (blockPool.height), it should never be removed by this check as we assume that
      correct peers respond timely and reliably. However, if a request is sent to a faulty peer then we 
      might get response on time or not, which is modelled with nondeterministic isTimedOut flag.
      See scheduler.go
      https://github.com/tendermint/tendermint/blob/4298bbcc4e25be78e3c4f21979d6aa01aede6e87/blockchain/v2/scheduler.go#L617
    *)
    LET nextHeightPeer == bPool.pendingBlocks[bPool.height] IN
    LET prunablePeers ==
        IF /\ nextHeightPeer /= NilPeer
           /\ nextHeightPeer \in FAULTY
           /\ isTimedOut
        THEN toRemovePeers \union {nextHeightPeer}
        ELSE toRemovePeers
    IN
    RemovePeers(prunablePeers, bPool)


\* Handle SyncTimeout. It models if progress has been made (height has increased) since the last SyncTimeout event.
HandleSyncTimeout(bPool) ==
    [bPool EXCEPT
            !.syncedBlocks = bPool.height - bPool.syncHeight,
            !.syncHeight = bPool.height
    ]

HandleResponse(msg, bPool) ==
    IF msg.type = "blockResponse" THEN
      HandleBlockResponse(msg, bPool)
    ELSE IF msg.type = "noBlockResponse" THEN
      HandleNoBlockResponse(msg, bPool)
    ELSE IF msg.type = "statusResponse" THEN
      HandleStatusResponse(msg, bPool)
    ELSE IF msg.type = "addPeer" THEN
      AddPeer(msg.peerId, bPool)
    ELSE IF msg.type = "removePeer" THEN
      RemovePeers({msg.peerId}, bPool)
    ELSE IF msg.type = "syncTimeout" THEN
      HandleSyncTimeout(bPool)
    ELSE
      bPool


(*
   At every node step we executed the following steps (atomically):
    1) input message is consumed and the corresponding handler is called,
    2) pruning logic is called
    3) block execution is triggered (we try to execute block at next height)
    4) a request to a peer is made (if possible) and
    5) we decide if termination condition is satisifed so we stop.
*)
NodeStep ==
   \E suspectedSet \in SUBSET AllPeerIds:                        \* suspectedSet is a nondeterministic set of peers
     \E isTimedOut \in BOOLEAN:
        LET bPool == HandleResponse(inMsg, blockPool) IN
        LET bp == TryPrunePeer(bPool, suspectedSet, isTimedOut) IN
        LET nbPool == ExecuteBlocks(bp) IN
        LET msgAndPool == CreateRequest(nbPool) IN
        LET nstate == ComputeNextState(msgAndPool.pool) IN

        /\ state' = nstate
        /\ blockPool' = msgAndPool.pool
        /\ outMsg' = msgAndPool.msg
        /\ inMsg' = AsInMsg(NoMsg)


\* If node is running, then in every step we try to create blockRequest.
\* In addition, input message (if exists) is consumed and processed.
NextNode ==
    \/ /\ state = "running"
       /\ NodeStep

    \/ /\ state = "finished"
       /\ UNCHANGED <<nvars, inMsg, outMsg>>


(********************************** Peers ***********************************)

InitPeers ==
    \E pHeights \in [AllPeerIds -> Heights]:
        peersState = [
         peerHeights |-> pHeights,
         statusRequested |-> FALSE,
         blocksRequested |-> AsOutMsgSet({})
    ]

HandleStatusRequest(msg, pState) ==
    [pState EXCEPT
        !.statusRequested = TRUE
    ]

HandleBlockRequest(msg, pState) ==
    [pState EXCEPT
        !.blocksRequested = pState.blocksRequested \union AsOutMsgSet({msg})
    ]

HandleRequest(msg, pState) ==
    IF msg = AsOutMsg(NoMsg)
    THEN pState
    ELSE IF msg.type = "statusRequest"
         THEN HandleStatusRequest(msg, pState)
         ELSE HandleBlockRequest(msg, pState)

CreateStatusResponse(peer, pState, anyHeight) ==
    LET m ==
        IF peer \in CORRECT
        THEN AsInMsg([type |-> "statusResponse", peerId |-> peer, height |-> pState.peerHeights[peer]])
        ELSE AsInMsg([type |-> "statusResponse", peerId |-> peer, height |-> anyHeight]) IN

    [msg |-> m, peers |-> pState]

CreateBlockResponse(msg, pState, arbitraryBlock) ==
    LET m ==
        IF msg.peerId \in CORRECT
        THEN AsInMsg([type |-> "blockResponse", peerId |-> msg.peerId, block |-> CorrectBlock(msg.height)])
        ELSE AsInMsg([type |-> "blockResponse", peerId |-> msg.peerId, block |-> arbitraryBlock]) IN
    LET npState ==
        [pState EXCEPT
            !.blocksRequested = pState.blocksRequested \ {msg}
        ] IN
    [msg |-> m, peers |-> npState]

GrowPeerHeight(pState) ==
    \E p \in CORRECT:
        /\ pState.peerHeights[p] < MAX_HEIGHT
        /\ peersState' = [pState EXCEPT !.peerHeights[p] = @ + 1]
        /\ inMsg' = AsInMsg(NoMsg)

SendStatusResponseMessage(pState) ==
    /\ \E arbitraryHeight \in Heights:
        \E peer \in AllPeerIds:
            LET msgAndPeers == CreateStatusResponse(peer, pState, arbitraryHeight) IN
               /\ peersState' = msgAndPeers.peers
               /\ inMsg' = msgAndPeers.msg


SendAddPeerMessage ==
   \E peer \in AllPeerIds:
     inMsg' = AsInMsg([type |-> "addPeer", peerId |-> peer])

SendRemovePeerMessage ==
   \E peer \in AllPeerIds:
     inMsg' = AsInMsg([type |-> "removePeer", peerId |-> peer])

SendSyncTimeoutMessage ==
     inMsg' = AsInMsg([type |-> "syncTimeout"])


SendControlMessage ==
    \/ SendAddPeerMessage
    \/ SendRemovePeerMessage
    \/ SendSyncTimeoutMessage

\* An extremely important property of block hashes (blockId):
\* If the block hash coincides with the hash of the reference block,
\* then the blocks should be equal.
UnforgeableBlockId(height, block) ==
    block.hashEqRef => block = chain[height]

\* A faulty peer cannot forge enough of the validators signatures.
\* In other words: If a commit contains enough signatures from the validators (in reality 2/3, in the model all), 
\* then the blockID points to the block on the chain, encoded as block.lastCommit.blockIdEqRef being true
\* A more precise rule should have checked that the commiters have over 2/3 of the VS's voting power.
NoFork(height, block) ==
    (height > 1 /\ block.lastCommit.committers = chain[height - 1].VS)
        => block.lastCommit.blockIdEqRef

\* Can be block produced by a faulty peer, assuming it cannot generate forks (basic assumption of the protocol)
IsBlockByFaulty(height, block) ==
    /\ block.height = height
    /\ UnforgeableBlockId(height, block)
    /\ NoFork(height, block)

SendBlockResponseMessage(pState) ==
    \* a response to a requested block: either by a correct, or by a faulty peer
    \/  /\ pState.blocksRequested /= AsOutMsgSet({})
        /\ \E msg \in pState.blocksRequested:
             \E block \in Blocks:
                 /\ IsBlockByFaulty(msg.height, block)
                 /\ LET msgAndPeers == CreateBlockResponse(msg, pState, block) IN
                    /\ peersState' = msgAndPeers.peers
                    /\ inMsg' = msgAndPeers.msg

    \* a faulty peer can always send an unsolicited block
    \/ \E peerId \in FAULTY:
         \E block \in Blocks:
           /\ IsBlockByFaulty(block.height, block)
           /\ peersState' = pState
           /\ inMsg' = AsInMsg([type |-> "blockResponse",
                                peerId |-> peerId, block |-> block])
        
SendNoBlockResponseMessage(pState) == 
    /\ peersState' = pState
    /\ inMsg' \in AsInMsgSet([type: {"noBlockResponse"}, peerId: FAULTY, height: Heights])
        
                   
SendResponseMessage(pState) == 
    \/  SendBlockResponseMessage(pState)
    \/  SendNoBlockResponseMessage(pState)
    \/  SendStatusResponseMessage(pState)
    

NextEnvStep(pState) ==
    \/  SendResponseMessage(pState)
    \/  GrowPeerHeight(pState)
    \/  SendControlMessage /\ peersState' = pState
        \* note that we propagate pState that was missing in the previous version


\* Peers consume a message and update it's local state. It then makes a single step, i.e., it sends at most single message.
\* Message sent could be either a response to a request or faulty message (sent by faulty processes).
NextPeers ==
    LET pState == HandleRequest(outMsg, peersState) IN
    /\ outMsg' = AsOutMsg(NoMsg)
    /\ NextEnvStep(pState)


\* the composition of the node, the peers, the network and scheduler
Init ==
    /\ IsCorrectChain(chain)   \* initialize the blockchain
    /\ InitNode
    /\ InitPeers
    /\ turn = "Peers"
    /\ inMsg = AsInMsg(NoMsg)
    /\ outMsg = AsOutMsg([type |-> "statusRequest"])

Next ==
  IF turn = "Peers"
  THEN
    /\ NextPeers
    /\ turn' = "Node"
    /\ UNCHANGED <<nvars, chain>>
  ELSE
    /\ NextNode
    /\ turn' = "Peers"
    /\ UNCHANGED <<peersState, chain>>


FlipTurn ==
 turn' =
  IF turn = "Peers" THEN
   "Node"
  ELSE
   "Peers"

\* Compute max peer height. Used as a helper operator in properties.
MaxCorrectPeerHeight(bPool) ==
    LET correctPeers == {p \in bPool.peerIds: p \in CORRECT} IN
    IF correctPeers = AsPidSet({})
    THEN 0 \* no peers, just return 0
    ELSE LET Hts == {bPool.peerHeights[p] : p \in correctPeers} IN
            CHOOSE max \in Hts: \A h \in Hts: h <= max

\* properties to check
TypeOK ==
    /\ state \in States
    /\ inMsg \in InMsgs
    /\ outMsg \in OutMsgs
    /\ turn \in {"Peers", "Node"}
    /\ peersState \in [
         peerHeights: [AllPeerIds -> Heights \union {NilHeight}],
         statusRequested: BOOLEAN,
         blocksRequested:
            SUBSET
               [type: {"blockRequest"}, peerId: AllPeerIds, height: Heights]

        ]
    /\ blockPool \in [
                height: Heights,
                peerIds: SUBSET AllPeerIds,
                peerHeights: [AllPeerIds -> Heights \union {NilHeight}],
                blockStore: [Heights -> Blocks \union {NilBlock}],
                receivedBlocks: [Heights -> AllPeerIds \union {NilPeer}],
                pendingBlocks: [Heights -> AllPeerIds \union {NilPeer}],
                syncedBlocks: Heights \union {NilHeight, -1},
                syncHeight: Heights
           ]

(* Incorrect synchronization: The last block may be never received *) 
Sync1 == 
    [](state = "finished" =>
        blockPool.height >= MaxCorrectPeerHeight(blockPool))

Sync1AsInv ==
    state = "finished" => blockPool.height >= MaxCorrectPeerHeight(blockPool)

(* Incorrect synchronization, as there may be a timeout *)
Sync2 ==
   \A p \in CORRECT:
        \/ p \notin blockPool.peerIds
        \/ [] (state = "finished" => blockPool.height >= blockPool.peerHeights[p] - 1)

Sync2AsInv ==
   \A p \in CORRECT:
        \/ p \notin blockPool.peerIds
        \/ (state = "finished" => blockPool.height >= blockPool.peerHeights[p] - 1)

(* Correct synchronization *)
Sync3 ==
   \A p \in CORRECT:
        \/ p \notin blockPool.peerIds
        \/ blockPool.syncedBlocks <= 0 \* timeout
        \/ [] (state = "finished" => blockPool.height >= blockPool.peerHeights[p] - 1)

Sync3AsInv ==
   \A p \in CORRECT:
        \/ p \notin blockPool.peerIds
        \/ blockPool.syncedBlocks <= 0 \* timeout
        \/ (state = "finished" => blockPool.height >= blockPool.peerHeights[p] - 1)

(* Naive termination *)
\* This property is violated, as the faulty peers may produce infinitely many responses
Termination ==
    WF_turn(FlipTurn) => <>(state = "finished")

(* Termination by timeout: the protocol terminates, if there is a timeout *)
\* the precondition: fair flip turn and eventual timeout when no new blocks were synchronized
TerminationByTOPre ==
  /\ WF_turn(FlipTurn)
  /\ <>(inMsg.type = "syncTimeout" /\ blockPool.height <= blockPool.syncHeight)

TerminationByTO ==
  TerminationByTOPre => <>(state = "finished")

(* The termination property when we only have correct peers *)
\* as correct peers may spam the node with addPeer, removePeer, and statusResponse,
\* we have to enforce eventual response (there are no queues in our spec)
CorrBlockResponse ==
  \A h \in Heights:
    [](outMsg.type = "blockRequest" /\ outMsg.height = h
            => <>(inMsg.type = "blockResponse" /\ inMsg.block.height = h))

\* a precondition for termination in presence of only correct processes
TerminationCorrPre ==
    /\ FAULTY = AsPidSet({})
    /\ WF_turn(FlipTurn)
    /\ CorrBlockResponse

\* termination when there are only correct processes    
TerminationCorr ==
    TerminationCorrPre => <>(state = "finished")

\* All synchronized blocks (but the last one) are exactly like in the reference chain
CorrectBlocksInv ==
    \/ state /= "finished"
    \/ \A h \in 1..(blockPool.height - 1):
        blockPool.blockStore[h] = chain[h]

\* A false expectation that the protocol only finishes with the blocks
\* from the processes that had not been suspected in being faulty
SyncFromCorrectInv ==
    \/ state /= "finished"
    \/ \A h \in 1..blockPool.height:
        blockPool.receivedBlocks[h] \in blockPool.peerIds \union {NilPeer}

\* A false expectation that a correct process is never removed from the set of peer ids.
\* A correct process may reply too late and then gets evicted.
CorrectNeverSuspectedInv ==
    CORRECT \subseteq blockPool.peerIds

BlockPoolInvariant ==
    \A h \in Heights:
      \* waiting for a block to arrive
      \/  /\ blockPool.receivedBlocks[h] = NilPeer
          /\ blockPool.blockStore[h] = NilBlock
      \* valid block is received and is present in the store
      \/  /\ blockPool.receivedBlocks[h] /= NilPeer
          /\ blockPool.blockStore[h] /= NilBlock
          /\ blockPool.pendingBlocks[h] = NilPeer

(* a few simple properties that trigger counterexamples *)

\* Shows execution in which peer set is empty
PeerSetIsNeverEmpty == blockPool.peerIds /= AsPidSet({})

\* Shows execution in which state = "finished" and MaxPeerHeight is not equal to 1
StateNotFinished ==
    state /= "finished" \/ MaxPeerHeight(blockPool) = 1


=============================================================================

\*=============================================================================
\* Modification History
\* Last modified Fri May 29 20:41:53 CEST 2020 by igor
\* Last modified Thu Apr 16 16:57:22 CEST 2020 by zarkomilosevic
\* Created Tue Feb 04 10:36:18 CET 2020 by zarkomilosevic
