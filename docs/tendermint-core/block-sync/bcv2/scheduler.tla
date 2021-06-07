------------------------------- MODULE scheduler -------------------------------
(*
 A specification of the fast sync scheduler that is introduced in blockchain/v2:
 
 https://github.com/tendermint/tendermint/tree/brapse/blockchain-v2-riri-reactor-2
 
 The model includes:
    - a scheduler that maintains the peers and blocks that it receives from the peers, and
    - one environment simulating a correct peer
    
 This specification focuses on the events that are received and produced by the scheduler.
 Communication between the scheduler and the other fastsync components is not specified.
*)

EXTENDS Integers, FiniteSets

\* the protocol parameters
CONSTANTS
    PeerIDs,        \* potential peer ids, a set of integers, e.g. 0..2
    ultimateHeight, \* the maximum height of the blockchain, an integer, e.g. 3
    numRequests     \* the maximum number of requests made when scheduling new blocks, e.g. 2

\* a few definitions
None == -1                      \* an undefined value
Heights == 0..ultimateHeight    \* potential heights
noErr == "errNone"
Errors == {
  noErr, "errPeerNotFound", "errDelRemovedPeer", "errAddDuplicatePeer",
  "errUpdateRemovedPeer", "errAfterPeerRemove", "errBadPeer", "errBadPeerState",
  "errProcessedBlockEv", "finished", "timeout", "errAddRemovedPeer", "errPeerNoBlock", "errBadBlockState"}

PeerStates == {"peerStateUnknown", "peerStateNew", "peerStateReady", "peerStateRemoved"}

BlockStates == {
  "blockStateUnknown", "blockStateNew", "blockStatePending",
  "blockStateReceived", "blockStateProcessed"}

\* basic stuff
Min(a, b) == IF a < b THEN a ELSE b
Max(a, b) == IF a > b THEN a ELSE b

\* the state of the scheduler:
VARIABLE  turn          \* who makes a step: the scheduler or the environment

\* the state of the reactor: 
VARIABLES inEvent,       \* an event from the environment to the scheduler
          envRunning \* a Boolean, negation of stopProcessing in the implementation
                    
\* the state of the scheduler:
VARIABLES outEvent,       \* an event from the scheduler to the environment
          scRunning

\* the block pool: 
VARIABLE scheduler
    (*
       scheduler is a record that contains: 
         height: Int,
            height of the next block to collect
         peers: PeerIDs,
            the set of peers that have connected in the past, may include removed peers
         peerHeights: [PeerIDs -> Heights],
            a map to collect the peer heights, >0 if peer in ready state, None (-1) otherwise
         peerStates: [PeerIDs -> PeerStates],
            a map to record the peer states 
         blockStates: [Heights -> BlockStates]
            a set of heights for which blocks are to be scheduled, pending or received
         pendingBlocks: [Heights -> PeerIDs],
            a set of heights for which blocks are to be scheduled or pending
         receivedBlocks: [Heights -> PeerIDs],
            a set of heights for which blocks were received but not yet processed
         blocks: Heights,
            the full set of blocks requested or downloaded by the scheduler
    *)
    
vars == <<turn, envRunning, inEvent, scRunning, outEvent, scheduler>>    

\* for now just keep the height in the block
Blocks == [ height: Heights ]

noEvent == [type |-> "NoEvent"] 

InEvents == 
  {noEvent} \cup
  [type: {"rTrySchedule", "tNoAdvanceExp"}] \cup
  [type: {"bcStatusResponse"},    peerID: PeerIDs, height: Heights] \cup
  [type: {"bcBlockResponse"},     peerID: PeerIDs, height: Heights, block: Blocks] \cup
  [type: {"bcNoBlockResponse"},   peerID: PeerIDs, height: Heights] \cup
  [type: {"pcBlockProcessed"},    peerID: PeerIDs, height: Heights] \cup
  [type: {"pcBlockVerificationFailure"}, height: Heights, firstPeerID: PeerIDs, secondPeerID: PeerIDs] \cup
  [type: {"bcAddNewPeer"},        peerID: PeerIDs] \cup
  [type: {"bcRemovePeer"},        peerID: PeerIDs]

\* Output events produced by the scheduler.
\* Note: in v2 the status request is done by the reactor/ environment
OutEvents == 
  {noEvent} \cup
  [type: {"scPeerError"}, peerID: PeerIDs, error: Errors] \cup
  [type: {"scSchedulerFail"}, error: Errors] \cup
  [type: {"scBlockRequest"}, peerID: PeerIDs, height: Heights] \cup
  [type: {"scBlockReceived"}, peerID: PeerIDs, block: Blocks] \cup
  [type: {"scPeersPruned"}, pruned: SUBSET [peerID: PeerIDs]] \cup 
  [type: {"scFinishedEv"},  error: Errors]
                        
(* ----------------------------------------------------------------------------------------------*)
(* The behavior of the scheduler that keeps track of peers, block requests and responses, etc.   *)
(* See scheduler.go                                                                              *)
(* https://github.com/tendermint/tendermint/blob/v0.33.3/blockchain/v2/scheduler.go              *)
(* ----------------------------------------------------------------------------------------------*)

addPeer(sc, peerID) == 
  IF peerID \in sc.peers THEN
    [err |-> "errAddDuplicatePeer", val |-> sc]
  ELSE IF sc.peerStates[peerID] = "peerStateRemoved" THEN
    [err |-> "errAddRemovedPeer", val |-> sc]
  ELSE
    LET newPeers == sc.peers \cup { peerID } IN
    LET newPeerHeights == [sc.peerHeights EXCEPT ![peerID] = None] IN
    LET newPeerStates == [sc.peerStates EXCEPT ![peerID] = "peerStateNew"] IN
    LET newSc == [sc EXCEPT 
      !.peers = newPeers,
      !.peerHeights = newPeerHeights, 
      !.peerStates = newPeerStates] IN
    [err |-> noErr, val |-> newSc]

maxHeight(states, heights) ==
 LET activePeers == {p \in DOMAIN states: states[p] = "peerStateReady"} IN
 IF activePeers = {} THEN
   0 \* no peers, just return 0
 ELSE 
   CHOOSE max \in { heights[p] : p \in activePeers }:
        \A p \in activePeers: heights[p] <= max \* max is the maximum

maxHeightScheduler(sc) ==
 maxHeight(sc.peerStates, sc.peerHeights)

removePeer(sc, peerID) ==
  IF peerID \notin sc.peers THEN
    [err |-> "errPeerNotFound", val |-> sc]
  ELSE IF sc.peerStates[peerID] = "peerStateRemoved" THEN
    [err |-> "errDelRemovedPeer", val |-> sc]
  ELSE
    LET newSc == [sc EXCEPT 
      !.peerHeights[peerID] = None,
      !.peerStates[peerID] = "peerStateRemoved",
      \* remove all blocks from peerID and block requests to peerID, see scheduler.removePeer
      !.blockStates = [h \in Heights |->
           IF sc.pendingBlocks[h] = peerID \/ sc.receivedBlocks[h] = peerID THEN "blockStateNew"
           ELSE sc.blockStates[h]],
      !.pendingBlocks = [h \in Heights |-> IF sc.pendingBlocks[h] = peerID THEN None ELSE sc.pendingBlocks[h]],
      !.receivedBlocks = [h \in Heights |-> IF sc.receivedBlocks[h] = peerID THEN None ELSE sc.receivedBlocks[h]]
      ] IN                 
    [err |-> noErr, val |-> newSc]

addNewBlocks(sc, newMph) ==
  \* add new blocks to be requested (e.g. when overall max peer height has changed)
  IF Cardinality(sc.blocks) >= numRequests THEN
    [newBlocks |-> sc.blocks, newBlockStates |-> sc.blockStates]
  ELSE
    LET requestSet == sc.height.. Min(numRequests+sc.height, newMph) IN
    LET heightsToRequest == {h \in requestSet: sc.blockStates[h] = "blockStateUnknown"} IN
    LET newBlockStates == [
       h \in Heights |-> IF h \in heightsToRequest THEN "blockStateNew"
          ELSE sc.blockStates[h]] IN
    [newBlocks |-> heightsToRequest, newBlockStates |-> newBlockStates]
 
\* Update the peer height (the peer should have been previously added)                    
setPeerHeight(sc, peerID, height) ==
  IF peerID \notin sc.peers THEN
    [err |-> "errPeerNotFound", val |-> sc]
  ELSE IF sc.peerStates[peerID] = "peerStateRemoved" THEN
    [err |-> "errUpdateRemovedPeer", val |-> sc]
  ELSE IF height < sc.peerHeights[peerID] THEN (* The peer is corrupt? Remove the peer. *)
    removePeer(sc, peerID)
  ELSE
    LET newPeerHeights == [sc.peerHeights EXCEPT ![peerID] = height] IN \* set the peer's height
    LET newPeerStates == [sc.peerStates EXCEPT ![peerID] = "peerStateReady"] IN \* set the peer's state
    LET newMph == maxHeight(newPeerStates, newPeerHeights) IN
    LET res == addNewBlocks(sc, newMph) IN
    LET newSc == [sc EXCEPT 
      !.peerHeights = newPeerHeights, !.peerStates = newPeerStates,
      !.blocks = res.newBlocks, !.blockStates = res.newBlockStates] IN
    [err |-> noErr, val |-> newSc]
    
nextHeightToSchedule(sc) ==
  LET toBeScheduled ==  {h \in DOMAIN sc.blockStates: sc.blockStates[h] = "blockStateNew"} \cup {ultimateHeight+1} IN
  CHOOSE minH \in toBeScheduled: \A h \in toBeScheduled: h >= minH

getStateAtHeight(sc, h) ==
  IF h < sc.height THEN
    "blockStateProcessed"
  ELSE IF h \in DOMAIN sc.blockStates THEN
    sc.blockStates[h]
  ELSE 
    "blockStateUnknown"
    
markPending(sc, peerID, h) ==
  IF getStateAtHeight(sc, h) /= "blockStateNew" THEN
    [err |-> "errBadBlockState", val |-> sc]
  ELSE IF peerID \notin sc.peers \/ sc.peerStates[peerID] /= "peerStateReady" THEN
    [err |-> "errBadPeerState", val |-> sc]
  ELSE IF h > sc.peerHeights[peerID] THEN
    [err |-> "errPeerTooShort", val |-> sc]
  ELSE
    LET newSc == [sc EXCEPT
      !.blockStates = [sc.blockStates EXCEPT ![h] = "blockStatePending"],
      !.pendingBlocks = [sc.pendingBlocks EXCEPT ![h] = peerID]] IN
    [err |-> noErr, val |-> newSc]

markReceived(sc, peerID, h) ==
  IF peerID \notin sc.peers \/ sc.peerStates[peerID] /= "peerStateReady" THEN
    [err |-> "errBadPeerState", val |-> sc]
  ELSE IF getStateAtHeight(sc, h) /= "blockStatePending" \/ sc.pendingBlocks[h] /= peerID THEN
    [err |-> "errBadPeer", val |-> sc]
  ELSE
    LET newSc == [sc EXCEPT
      !.blockStates = [sc.blockStates EXCEPT ![h] = "blockStateReceived"],
      !.pendingBlocks = [sc.pendingBlocks EXCEPT ![h] = None],
      !.receivedBlocks = [sc.receivedBlocks EXCEPT ![h] = peerID]] IN
    [err |-> noErr, val |-> newSc]

markProcessed(sc, h) ==
  IF getStateAtHeight(sc, h) /= "blockStateReceived" THEN
    [err |-> "errProcessedBlockEv", val |-> sc]
  ELSE
    LET newSc == [sc EXCEPT
      !.blockStates = [sc.blockStates EXCEPT ![h] = "blockStateProcessed"],
      !.receivedBlocks = [sc.receivedBlocks EXCEPT ![h] = None],
      !.height = sc.height + 1] IN
    [err |-> noErr, val |-> newSc]

reachedMaxHeight(sc) ==
  IF sc.peers = {} THEN
    FALSE
  ELSE
    LET maxH == maxHeightScheduler(sc) IN
    maxH > 0 /\ (sc.height >= maxH)

highPeers(sc, minH) == {p \in sc.peers: sc.peerHeights[p] >= minH}

(* ----------------------------------------------------------------------------------------------*)
(* The behavior of the scheduler state machine                                                   *)
(* See scheduler.go                                                                              *)
(* https://github.com/tendermint/tendermint/tree/brapse/blockchain-v2-riri-reactor-2/scheduler.go*)
(* ----------------------------------------------------------------------------------------------*)
blStateInit(h, start) ==
  IF h <= start THEN
    "blockStateProcessed"
  ELSE "blockStateUnknown"

InitSc ==
  /\ scRunning = TRUE
  /\ outEvent = noEvent
  /\ \E startHeight \in Heights:
    scheduler = [
      initHeight |-> startHeight,
      height |-> startHeight + 1,
      peers |-> {},
      peerHeights |-> [p \in PeerIDs |-> None],
      peerStates |-> [p \in PeerIDs |-> "peerStateUnknown"],
      blocks |-> {},
      blockStates |-> [h \in Heights |-> blStateInit(h, startHeight)],
      pendingBlocks |-> [h \in Heights |-> None],
      receivedBlocks |-> [h \in Heights |-> None]
      ]
     
handleAddNewPeer ==
  /\ inEvent.type = "bcAddNewPeer"
  /\ LET res == addPeer(scheduler, inEvent.peerID) IN
     IF res.err /= noErr THEN
       /\ outEvent' = [type |-> "scSchedulerFail", error |-> res.err]
       /\ UNCHANGED <<scheduler>>
     ELSE
       /\ scheduler' = res.val
       /\ UNCHANGED outEvent
  /\ UNCHANGED scRunning

finishSc(event) ==
  event.type = "scFinishedEv" /\ event.error = "finished"
       
handleRemovePeer ==
  /\ inEvent.type = "bcRemovePeer"
  /\ LET res == removePeer(scheduler, inEvent.peerID) IN
     IF res.err /= noErr THEN
       /\ outEvent' = [type |-> "scSchedulerFail", error |-> res.err]
       /\ UNCHANGED scheduler
     ELSE
       /\ scheduler' = res.val
       /\ IF reachedMaxHeight(scheduler') THEN
            outEvent' = [type |-> "scFinishedEv", error |-> "errAfterPeerRemove"]
          ELSE 
            UNCHANGED outEvent
  /\ IF finishSc(outEvent') THEN
       scRunning' = FALSE
     ELSE UNCHANGED scRunning

handleStatusResponse ==
  /\ inEvent.type = "bcStatusResponse"
  /\ LET res == setPeerHeight(scheduler, inEvent.peerID, inEvent.height) IN
     IF res.err /= noErr THEN
       /\ outEvent' = [type |-> "scPeerError", peerID |-> inEvent.peerID, error |-> res.err]
       /\ UNCHANGED scheduler
     ELSE
       /\ scheduler' = res.val
       /\ UNCHANGED outEvent
  /\ UNCHANGED scRunning

handleTrySchedule == \* every 10 ms, but our spec is asynchronous
  /\ inEvent.type = "rTrySchedule"
  /\ LET minH == nextHeightToSchedule(scheduler) IN
     IF minH = ultimateHeight+1 THEN
       /\ outEvent' = noEvent
       /\ UNCHANGED scheduler
     ELSE IF minH = ultimateHeight+1 THEN
       /\ outEvent' = noEvent
       /\ UNCHANGED scheduler
     ELSE
       /\ LET hp == highPeers(scheduler, minH) IN
          IF hp = {} THEN
            /\ outEvent' = noEvent
            /\ UNCHANGED scheduler
          ELSE \E bestPeerID \in hp:
            /\ LET res == markPending(scheduler, bestPeerID, minH) IN
               /\ IF res.err /= noErr THEN
                    outEvent' = [type |-> "scSchedulerFail", error|-> res.err]
                  ELSE
                    outEvent' = [type |-> "scBlockRequest", peerID |-> bestPeerID, height |-> minH]
               /\ scheduler' = res.val
  /\ UNCHANGED scRunning

handleBlockResponse ==
  /\ inEvent.type = "bcBlockResponse"
  /\ LET res == markReceived(scheduler, inEvent.peerID, inEvent.height) IN
     IF res.err /= noErr THEN
       LET res1 == removePeer(scheduler, inEvent.peerID) IN
       /\ outEvent' = [type |-> "scPeerError", peerID |-> inEvent.peerID, error |-> res.err]
       /\ scheduler' = res1.val
     ELSE
       /\ outEvent' = [type |-> "scBlockReceived", peerID |-> inEvent.peerID, block |-> inEvent.block]
       /\ scheduler' = res.val
  /\ UNCHANGED scRunning

handleNoBlockResponse ==
  /\ inEvent.type = "bcNoBlockResponse"
  /\ IF (scheduler.peers = {} \/ scheduler.peerStates[inEvent.peerID] = "peerStateRemoved") THEN
        /\ outEvent' = noEvent
        /\ UNCHANGED scheduler
     ELSE
       LET res == removePeer(scheduler, inEvent.peerID) IN
        /\ outEvent' = [type |-> "scPeerError", peerID |-> inEvent.peerID, error |-> "errPeerNoBlock"]
        /\ scheduler' = res.val
  /\ UNCHANGED scRunning

handleBlockProcessed ==
  /\ inEvent.type = "pcBlockProcessed"
  /\ IF inEvent.height /= scheduler.height THEN
       /\ outEvent' = [type |-> "scSchedulerFail", error |-> "errProcessedBlockEv"]
       /\ UNCHANGED scheduler
     ELSE
       LET res == markProcessed(scheduler, inEvent.height) IN
       IF res.err /= noErr THEN
         /\ outEvent' = [type |-> "scSchedulerFail", error |-> res.err]
         /\ UNCHANGED scheduler
       ELSE
         /\ scheduler' = res.val
         /\ IF reachedMaxHeight(scheduler') THEN
              outEvent' = [type |-> "scFinishedEv", error |-> "finished"]
            ELSE
              outEvent' = noEvent
  /\ IF finishSc(outEvent') THEN
       scRunning' = FALSE
     ELSE UNCHANGED scRunning

handleBlockProcessError ==
  /\ inEvent.type = "pcBlockVerificationFailure"
  /\ IF scheduler.peers = {} THEN
        /\ outEvent' = noEvent
        /\ UNCHANGED scheduler
     ELSE
        LET res1 == removePeer(scheduler, inEvent.firstPeerID) IN
        LET res2 == removePeer(res1.val, inEvent.secondPeerID) IN
          /\ IF reachedMaxHeight(res2.val) THEN
               outEvent' = [type |-> "scFinishedEv", error |-> "finished"]
             ELSE
               outEvent' = noEvent
          /\ scheduler' = res2.val
  /\ IF finishSc(outEvent') THEN
       scRunning' = FALSE
     ELSE UNCHANGED scRunning

handleNoAdvanceExp ==
    /\ inEvent.type = "tNoAdvanceExp"
    /\ outEvent' = [type |-> "scFinishedEv", error |-> "timeout"]
    /\ scRunning' = FALSE
    /\ UNCHANGED <<scheduler>>

NextSc ==
  IF ~scRunning THEN
    UNCHANGED <<outEvent, scRunning, scheduler>>
  ELSE
    \/ handleStatusResponse
    \/ handleAddNewPeer
    \/ handleRemovePeer
    \/ handleTrySchedule
    \/ handleBlockResponse
    \/ handleNoBlockResponse
    \/ handleBlockProcessed
    \/ handleBlockProcessError
    \/ handleNoAdvanceExp

(* ----------------------------------------------------------------------------------------------*)
(* The behavior of the environment.                                                              *)
(* ----------------------------------------------------------------------------------------------*)

InitEnv ==
  /\ inEvent = noEvent
  /\ envRunning = TRUE

OnGlobalTimeoutTicker ==
  /\ inEvent' = [type |-> "tNoAdvanceExp"]
  /\ envRunning' = FALSE

OnTrySchedule ==
  /\ inEvent' = [type |-> "rTrySchedule"]
  /\ UNCHANGED envRunning

OnAddPeerEv ==
  /\ inEvent' \in [type: {"bcAddNewPeer"}, peerID: PeerIDs]
  /\ UNCHANGED envRunning

OnStatusResponseEv ==
  \* any status response can come from the blockchain, pick one non-deterministically
  /\ inEvent' \in [type: {"bcStatusResponse"}, peerID: PeerIDs, height: Heights]
  /\ UNCHANGED envRunning

OnBlockResponseEv ==
  \* any block response can come from the blockchain, pick one non-deterministically
  /\ inEvent' \in [type: {"bcBlockResponse"}, peerID: PeerIDs, height: Heights, block: Blocks]
  /\ UNCHANGED envRunning

OnNoBlockResponseEv ==
  \* any no block response can come from the blockchain, pick one non-deterministically
  /\ inEvent' \in [type: {"bcNoBlockResponse"}, peerID: PeerIDs, height: Heights]
  /\ UNCHANGED envRunning

OnRemovePeerEv ==
  \* although bcRemovePeer admits an arbitrary set, we produce just a singleton
  /\ inEvent' \in [type: {"bcRemovePeer"}, peerID: PeerIDs]
  /\ UNCHANGED envRunning

OnPcBlockProcessed ==
  /\ inEvent' \in [type: {"pcBlockProcessed"}, peerID: PeerIDs, height: Heights]
  /\ UNCHANGED envRunning

OnPcBlockVerificationFailure ==
  /\ inEvent' \in [type: {"pcBlockVerificationFailure"}, firstPeerID: PeerIDs, secondPeerID: PeerIDs, height: Heights]
  /\ UNCHANGED envRunning

\* messages from scheduler
OnScFinishedEv ==
  /\ outEvent.type = "scFinishedEv"
  /\ envRunning' = FALSE \* stop the env
  /\ UNCHANGED inEvent

NextEnv ==
  IF ~envRunning THEN
    UNCHANGED <<inEvent, envRunning>>
  ELSE
    \/ OnScFinishedEv
    \/ OnGlobalTimeoutTicker
    \/ OnAddPeerEv
    \/ OnTrySchedule
    \/ OnStatusResponseEv
    \/ OnBlockResponseEv
    \/ OnNoBlockResponseEv
    \/ OnRemovePeerEv
    \/ OnPcBlockProcessed
    \/ OnPcBlockVerificationFailure

(* ----------------------------------------------------------------------------------------------*)
(* The system is the composition of the environment and the schedule                             *)
(* ----------------------------------------------------------------------------------------------*)
Init == turn = "environment" /\ InitEnv /\ InitSc

FlipTurn == 
  turn' = (
    IF turn = "scheduler" THEN 
      "environment" 
    ELSE
      "scheduler"
  ) 

\* scheduler and environment alternate their steps (synchronous composition introduces more states)
Next ==
/\ FlipTurn
/\ IF turn = "scheduler" THEN
     /\ NextSc
     /\ inEvent' = noEvent
     /\ UNCHANGED envRunning
   ELSE
     /\ NextEnv
     /\ outEvent' = noEvent
     /\ UNCHANGED <<scRunning, scheduler>>

Spec == Init /\ [][Next]_vars /\ WF_turn(FlipTurn)

(* ----------------------------------------------------------------------------------------------*)
(* Invariants                                                                                    *)
(* ----------------------------------------------------------------------------------------------*)
TypeOK ==
    /\ turn \in {"scheduler", "environment"}
    /\ inEvent \in InEvents
    /\ envRunning \in BOOLEAN
    /\ outEvent \in OutEvents
    /\ scheduler \in [
         initHeight: Heights,
         height: Heights \cup {ultimateHeight + 1},
         peers: SUBSET PeerIDs,
         peerHeights: [PeerIDs -> Heights \cup {None}],
         peerStates: [PeerIDs -> PeerStates],
         blocks: SUBSET Heights,
         blockStates: [Heights -> BlockStates],
         pendingBlocks: [Heights -> PeerIDs \cup {None}],
         receivedBlocks: [Heights -> PeerIDs \cup {None}]
       ]

(* ----------------------------------------------------------------------------------------------*)
(* Helpers for Properties                                                                        *)
(* ----------------------------------------------------------------------------------------------*)
NoFailuresAndTimeouts ==
    /\ inEvent.type /= "bcRemovePeer"
    /\ inEvent.type /= "bcNoBlockResponse"
    /\ inEvent.type /= "pcBlockVerificationFailure"

\* simulate good peer behavior using this formula. Useful to show termination in the presence of good peers.
GoodResponse ==
  \/ inEvent.type
         \in {"bcAddNewPeer", "bcStatusResponse", "bcBlockResponse", "pcBlockProcessed", "rTrySchedule"}
  \/ ~envRunning

\* all blocks from initHeight up to max peer height have been processed
AllRequiredBlocksProcessed ==
  LET maxH == Max(scheduler.height, maxHeightScheduler(scheduler)) IN
  LET processedBlocks == {h \in scheduler.initHeight.. maxH-1: scheduler.blockStates[h] = "blockStateProcessed"} IN
  scheduler.height >= maxH /\ Cardinality(processedBlocks) = scheduler.height - scheduler.initHeight

IncreaseHeight ==
  (scheduler'.height > scheduler.height) \/ (scheduler.height >= ultimateHeight)

(* ----------------------------------------------------------------------------------------------*)
(* Expected properties                                                                           *)
(* ----------------------------------------------------------------------------------------------*)
(*                                                                                               *)
(* 1. TerminationWhenNoAdvance - termination if there are no correct peers.                      *)
(* The model assumes the "noAdvance" timer expires and the "tNoAdvanceExp" event is received     *)
(*                                                                                               *)
TerminationWhenNoAdvance ==
  (inEvent.type = "tNoAdvanceExp")
    => <>(outEvent.type = "scFinishedEv" /\ outEvent.error = "timeout")

(* ----------------------------------------------------------------------------------------------*)
(*                                                                                               *)
(* 2. TerminationGoodPeers -                                                                     *)
(*     termination when IncreaseHeight holds true, fastsync is progressing, all blocks processed *)
(*                                                                                               *)
TerminationGoodPeers ==
 (/\ scheduler.height < ultimateHeight
  /\ <>[]GoodResponse
  /\[]<>(<<IncreaseHeight>>_<<scheduler, turn>>)
 )
    => <>(outEvent.type = "scFinishedEv" /\ AllRequiredBlocksProcessed)

(* This property is violated. It shows that the precondition of TerminationGoodPeers is not      *)
(* always FALSE                                                                                  *)
TerminationGoodPeersPre ==
 (/\ scheduler.height < ultimateHeight
  /\ <>[]GoodResponse
  /\[]<>(<<IncreaseHeight>>_<<scheduler, turn>>)
 )
    => FALSE

(* ----------------------------------------------------------------------------------------------*)
(* 3. TerminationAllCases -                                                                      *)
(*     any peer behavior, either terminates with all blocks processed or times out               *)
TerminationAllCases ==
 (/\ scheduler.height < ultimateHeight
  /\(([]<> (<<IncreaseHeight>>_<<scheduler, turn>>)) \/ <>(inEvent.type = "tNoAdvanceExp"))
 )
    => <>(outEvent.type = "scFinishedEv" /\ (AllRequiredBlocksProcessed \/ outEvent.error = "timeout"))

(* This property is violated. It shows that the precondition of TerminationAllCases is not       *)
(* always FALSE                                                                                  *)
TerminationAllCasesPre ==
 (/\ scheduler.height < ultimateHeight
  /\(([]<> (<<IncreaseHeight>>_<<scheduler, turn>>)) \/ <>(inEvent.type = "tNoAdvanceExp"))
 )
    => FALSE

(* This property is violated. TLC output shows an example of increasing heights in the scheduler *)
SchedulerIncreasePre ==
[]<>(<<IncreaseHeight>>_<<scheduler, turn>>)
    => FALSE

=============================================================================
\* Modification History
\* Last modified Thu Apr 16 13:21:33 CEST 2020 by ancaz
\* Created Sat Feb 08 13:12:30 CET 2020 by ancaz
