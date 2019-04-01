# ADR 036: Blockchain Reactor Refactor  

## Changelog

19-03-2019: Initial draft

## Context

The Blockchain Reactor's high level responsibility is to enable peers who are
far behind the current state of the blockchain to quickly catch up by downloading
many blocks in parallel, verifying their commits, and executing them against the
ABCI application. The current architecture diagram of the blockchain reactor can be found here: 

![Blockchain Reactor Architecture Diagram](img/bc-reactor.png)

The major issue with this reactor is difficulty to understand the 
current design and the implementation, and the problem with writing tests.
More precisely, the current architecture consists of dozens of routines and it is tightly depending on the `Switch`, making writing unit tests almost impossible. Current tests require setting up complex dependency graphs and dealing with concurrency. Note that having dozens of routines is in this case overkill as most of the time routines sits idle waiting for something to happen (message to arrive or timeout to expire). Due to dependency on the `Switch`, 
testing relatively complex network scenarios and failures (for example adding and removing peers) is very complex tasks and frequently lead to complex tests with not deterministic behavior ([#3400]).   

This resulted in several issues (some are closed and some are still open): 
[#3400], [#2897], [#2896], [#2699], [#2888], [#2457], [#2622], [#2026].  

## Decision

To remedy these issues we plan a major refactor of the blockchain reactor. The proposed architecture is largely inspired by ADR-30 and is presented 
on the following diagram:
![Blockchain Reactor Refactor Diagram](img/bc-reactor-refactor.png)

We suggest a concurrency architecture where the core algorithm (we call it `Controller`) is extracted into a finite state machine. 
The active routine of the reactor is called `Executor` and is responsible for receiving and sending messages from/to peers and triggering timeouts.
What messages should be sent and timeouts triggered is defined mostly by the `Controller`. The exception is peer heartbeat mechanism which is `Executor`
responsibility. The heartbeat mechanism consists of periodically sending `StatusRequest` messages to peers and signalling to `Executor` in case a peer does not respond 
timely with the corresponding `StatusReport` message that a peer is not responsive so it should be removed from the peer list.    
This simpler architecture is easier to understand and makes writing of unit tests simpler tasks as most of the critical
logic is part of the `Controller` function. 

### Implementation changes

We assume the following system model for "fast sync" protocol: 

* a node is connected to a random subset of all nodes that represents its peer set. Some nodes are correct and some might be faulty. We don't make assumptions about ratio of  
  faulty nodes, i.e., it is possible that all nodes in some peer set are faulty. 
* we assume that communication between correct nodes is synchronous, i.e., if a correct node `p` sends a message `m` to a correct node `q` at time `t`, then `q` will receive
  message the latest at time `t+Delta` where `Delta` is a system parameter that is known. `Delta` is normally order of magnitude higher than the real communication delay 
  (maximum) between correct nodes. Therefore if a correct node `p` sends a request message to a correct node `q` at time `t` and there is no the corresponding reply at time
  `t + 2*Delta`, then `p` can assume that `q` is faulty. Note that the network assumptions for the consensus reactor are different 
  (we assume partially synchronous model there). 

The "fast sync" protocol is formally specified as follows:

- `Correctness`: If a correct node `p` is connected to a correct node `q` for a long enough period of time, then `p` will eventually download all requested blocks from `q`.
- `Termination`: If a set of peers of a correct node `p` is stable (no new nodes are added to the peer set) for a long enough period of time, then protocol eventually terminates. 
- `Fairness`: A correct node `p` sends requests for blocks to all peers from its peer set.   

The `Controller` can be modelled as a function with clearly defined inputs:

* `State` - current state of the node. Contains data about connected peers and its behavior, pending requests, received blocks, etc.
* `Event` - significant events in the network.

producing clear outputs:

* `State` - updated state of the node,
* `Message` - signal what message to send,
* `TimeoutTrigger` - signal that timeout should be triggered.

```go
type Event int

const (
	EventUnknown Event = iota
	EventStatusReport
	EventBlockRequest
	EventNewBlock
	EventRemovePeer
	EventResponseTimeout
	EventTerminationTimeout
)

type BlockRequest struct {
	Height int64
	PeerID p2p.ID
}

type NewBlock struct {
	Height        int64
	Block         Block
	Commit        Commit
	PeerID        ID
	CurrentHeight int64
}

type StatusReport struct {
	PeerID ID
	Height int64
}

type RemovePeer struct {
	PeerID ID
	Height int64
}

type ResponseTimeout struct {
	PeerID ID
	Height int64
}

type TerminationTimeout struct {
	Height int64
}

type Message int

const (
	MessageUnknown Message = iota
	MessageBlockRequest
	MessageBlockResponse
	MessageStatusReport
)

type BlockRequestMessage struct {
	Height int64
	PeerID ID
}

type BlockResponseMessage struct {
	Height        int64
	Block         Block
	Commit        Commit
	PeerID        ID
	CurrentHeight int64
}

type StatusReportMessage struct {
	Height int64
}

type TimeoutTrigger int

const (
	TimeoutUnknown TimeoutTrigger = iota
	TimeoutResponseTrigger
	TimeoutTerminationTrigger
)
```

The Controller state machine can be in two modes (states): `FastSyncMode` when
it is trying to catch up with the network by downloading committed blocks,
and `ConsensusMode` in which it executes Tendermint consensus protocol. We
assume that "fast sync" mode terminates once the Controller switch to
`ConsensusMode`.  

``` go
type Mode int

const (
	ModeUnknown Mode = iota
	ModeFastSync
	ModeConsensus
)

type ControllerState struct {
	Height             int64            // the first block that is not committed
	Mode               Mode             // mode of operation of the state machine
	PeerMap            map[ID]PeerStats // map of peer IDs to peer statistics
	MaxRequestPending  int64            // maximum height of the pending requests
	FailedRequests     []int64          // list of failed block requests
	PendingRequestsNum int              // number of pending requests
	Store              []BlockInfo      // contains list of downloaded blocks
	Executor           BlockExecutor    // store, verify and executes blocks
}

type PeerStats struct {
	Height         int64
	PendingRequest int64 // it can be list in case there are multiple outstanding requests per peer
}

type BlockInfo struct {
	Block  Block
	Commit Commit
	PeerID ID // a peer from which we downloading the corresponding Block and Commit
}
```

The `Controller` is initialized by providing initial height (`startHeight`) from which it will start downloading blocks from peers.

``` go
func NewControllerState(startHeight int64, executor BlockExecutor) ControllerState {
	state = ControllerState {}
    state.Height = startHeight
	state.Mode = ModeFastSync
	state.MaxRequestPending = startHeight - 1
    state.PendingRequestsNum = 0
    state.Executor = executor
    initialize state.PeerMap, state.FailedRequests and state.Store to empty data structures
    return state
}

func handleEvent(state ControllerState, event Event) (ControllerState, Message, TimeoutTrigger, Error) {
	msg = nil
	timeout = nil
	error = nil

	switch state.Mode {
	case ModeConsensus:
		switch event := event.(type) {
		case EventBlockRequest:
			msg = createBlockResponseMessage(state, event)
			return state, msg, timeout, error
		default:
			error = "Only respond to BlockRequests while in ModeConsensus!"
			return state, msg, timeout, error
		}

	case ModeFastSync:
		switch event := event.(type) {
		case EventBlockRequest:
			msg = createBlockResponseMessage(state, event)
			return state, msg, timeout, error

		case EventStatusReport:
			if _, ok := state.PeerMap[event.PeerID]; !ok {
				peerStats = PeerStats{-1, -1}
			} else {
				peerStats = state.PeerMap[event.PeerID]
			}

			if event.Height > peerStats.Height {
				peerStats.Height = event.Height
			}
            // if there are no pending requests for this peer, try to send him a request for block
            if peerStats.PendingRequest == -1 {
				msg = createBlockRequestMessage(state, event.PeerID, peerStats.Height)
                // msg == nil if no request for block can be made to a peer at this point in time
                if msg != nil {
					peerStats.PendingRequest = msg.Height
                    state.PendingRequestsNum++
                    // when a request for a block is sent to a peer, a response timeout is triggered. If no corresponding block is sent by the peer 
                    // during response timeout period, then the peer is considered faulty and is removed from the peer set.
					timeout = ResponseTimeoutTrigger{ msg.PeerID, msg.Height, PeerTimeout }
				} else if state.PendingRequestsNum == 0 {
                    // if there are no pending requests and no new request can be placed to the peer, termination timeout is triggered.
                    // If termination timeout expires and we are still at the same height and there are no pending requests, the "fast-sync"
                    // mode is finished and we switch to `ModeConsensus`.
                    timeout = TerminationTimeoutTrigger{ state.Height, TerminationTimeout }
				}
			}
			state.PeerMap[event.PeerID] = peerStats
			return state, msg, timeout, error

		case EventRemovePeer:
			if _, ok := state.PeerMap[event.PeerID]; ok {
				pendingRequest = state.PeerMap[event.PeerID].PendingRequest
                // if a peer is removed from the peer set, its pending request is declared failed and added to the `FailedRequests` list 
                // so it can be retried. 
                if pendingRequest != -1 {
					add(state.FailedRequests, pendingRequest)
				}
				state.PendingRequestsNum--
				delete(state.PeerMap, event.PeerID)
                // if the peer set is empty after removal of this peer then termination timeout is triggered.
                if state.PeerMap.isEmpty() {
					timeout = TerminationTimeoutTrigger{state.Height, TerminationTimeout}
				}
			} else {
				error = "Removing unknown peer!"
			}
			return state, msg, timeout, error

		case EventNewBlock:
			if state.PeerMap[event.PeerID] {
				peerStats = state.PeerMap[event.PeerID]
                // when expected block arrives from a peer, it is added to the store so it can be verified and if correct executed after.
                if peerStats.PendingRequest == event.Height {
					peerStats.PendingRequest = -1
					state.PendingRequestsNum--
					if event.CurrentHeight > peerStats.Height {
						peerStats.Height = event.CurrentHeight
					}
					state.Store[event.Height] = BlockInfo{event.Block, event.Commit, event.PeerID}
                    // blocks are verified sequentially so adding a block to the store does not mean that it will be immediately verified
                    // as some of the previous blocks might be missing.
                    state = verifyBlocks(state) // it can lead to event.PeerID being removed from peer list

					if _, ok := state.PeerMap[event.PeerID]; ok {
						// we try to identify new request for a block that can be asked to the peer
                        msg = createBlockRequestMessage(state, event.PeerID, peerStats.Height)
                        if msg != nil {
							peerStats.PendingRequests = msg.Height
							state.PendingRequestsNum++
							// if request for block is made, response timeout is triggered
                            timeout = ResponseTimeoutTrigger{ msg.PeerID, msg.Height, PeerTimeout }
						} else if state.PeerMap.isEmpty() || state.PendingRequestsNum == 0 {
                            // if the peer map is empty (the peer can be removed as block verification failed) or there are no pending requests
                            // termination timeout is triggered.
                            timeout = TerminationTimeoutTrigger{ state.Height, TerminationTimeout }
						}
					}
				} else {
					error = "Received Block from wrong peer!"
				}
			} else {
				error = "Received Block from unknown peer!"
			}

			state.PeerMap[event.PeerID] = peerStats
			return state, msg, timeout, error

		case EventResponseTimeout:
			if _, ok := state.PeerMap[event.PeerID]; ok {
				peerStats = state.PeerMap[event.PeerID]
                // if a response timeout expires and the peer hasn't delivered the block, the peer is removed from the peer list and
                // the request is added to the `FailedRequests` so the block can be downloaded from other peer 
                if peerStats.PendingRequest == event.Height {
					add(state.FailedRequests, pendingRequest)
					delete(state.PeerMap, event.PeerID)
					state.PendingRequestsNum--
                    // if peer set is empty, then termination timeout is triggered
                    if state.PeerMap.isEmpty() {
						timeout = TimeoutTrigger{ state.Height, TerminationTimeout }
					}
				}
			}

			return state, msg, timeout, error

		case EventTerminationTimeout:
            // Termination timeout is triggered in case of empty peer set and in case there are no pending requests.
            // If this timeout expires and in the meantime no new peers are added or new pending requests are made
            // then "fast-sync" mode terminates by switching to `ModeConsensus`.
            // Note that termination timeout should be higher than the response timeout. 
            if state.Height == event.Height && state.PendingRequestsNum == 0 {
				state.State = ConsensusMode
			}
			return state, msg, timeout, error

		default:
			error = "Received unknown event type!"
			return state, msg, timeout, error
		}

	}
}

func createBlockResponseMessage(state ControllerState, event BlockRequest) BlockResponseMessage {
	msg = nil
	if _, ok := state.PeerMap[event.PeerID]; !ok {
		peerStats = PeerStats{-1, -1}
	}
	if state.Executor.ContainsBlockWithHeight(event.Height) && event.Height > peerStats.Height {
		peerStats = event.Height
		msg = BlockResponseMessage{
			Height:        event.Height,
			Block:         state.Executor.getBlock(eventHeight),
			Commit:        state.Executor.getCommit(eventHeight),
			PeerID:        event.PeerID,
			CurrentHeight: state.Height - 1,
		}
	}
	state.PeerMap[event.PeerID] = peerStats
	return msg
}

func createBlockRequestMessage(state ControllerState, peerID ID, peerHeight int64) BlockRequestMessage {
	msg = nil
	blockHeight = -1

    r = find request in state.FailedRequests such that r <= peerHeight // returns `nil` if there are no such request
    // if there is a height in failed requests that can be downloaded from the peer send request to it
    if r != nil {
		blockNumber = r
		delete(state.FailedRequests, r)
	} else if state.MaxRequestPending < peerHeight {    
        // if height of the maximum pending request is smaller than peer height, then ask peer for next block
        state.MaxRequestPending++
		blockHeight = state.MaxRequestPending // increment state.MaxRequestPending and then return the new value
	}

	if blockHeight > -1 {
		msg = BlockRequestMessage{ blockHeight, peerID }
	}
	return msg
}

func verifyBlocks(state State) State {
	done = false
	for !done {
		block = state.Store[height]

		if block != nil {
			verified = verify block.Block using block.Commit // return `true` is verification succeed, 'false` otherwise

			if verified {
				block.Execute()   // executing block is costly operation so it might make sense executing asynchronously
				state.Height++
			} else {
				// if block verification failed, then it is added to `FailedRequests` and the peer is removed from the peer set 
                add(state.FailedRequests, height)
                state.Store[height] = nil
				if _, ok := state.PeerMap[block.PeerID]; ok {
					pendingRequest = state.PeerMap[block.PeerID].PendingRequest
                    // if there is a pending request sent to the peer that is just to be removed from the peer set, add it to `FailedRequests`
                    if pendingRequest != -1 {
                        add(state.FailedRequests, pendingRequest)
                        state.MaxRequestPending--
					}
					delete(state.PeerMap, event.PeerID)
				}
				done = true
			}
		} else {
			done = true
		}
	}
	return state
}
```

In the proposed architecture `Controller` is not active task, i.e., it is being called by the `Executor`. Depending on the return values returned by `Controller`,
`Executor` will send a message to some peer (`msg` != nil), trigger a timeout (`timeout` != nil) or deal with errors (`error` != nil). 
In case a timeout is triggered, it will provide as an input to `Controller` the corresponding timeout event once timeout expires.   


## Status

Draft.

## Consequences

### Positive

- isolated implementation of the algorithm
- improved testability - simpler to prove correctness
- clearer separation of concerns - easier to reason

### Negative

### Neutral
