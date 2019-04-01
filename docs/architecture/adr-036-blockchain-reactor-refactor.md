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
type State int

const (
	StateUnknown State = iota
	StateFastSyncMode
	StateConsensusMode
)

type ControllerState struct {
	Height             int64            // the first block that is not committed
	State              State            // mode of operation of the state machine
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
func ControllerInit(state ControllerState, startHeight int64) ControllerState {
	state.Height = startHeight
	state.State = FastSyncMode
	state.MaxRequestPending = startHeight - 1
	state.PendingRequestsNum = 0
}

func ControllerHandle(event Event, state ControllerState) (ControllerState, Message, TimeoutTrigger, Error) {
	msg = nil
	timeout = nil
	error = nil

	switch state.State {
	case StateConsensusMode:
		switch event := event.(type) {
		case EventBlockRequest:
			msg = createBlockResponseMessage(state, event)
			return state, msg, timeout, error
		default:
			error = "Only respond to BlockRequests while in ConsensusMode!"
			return state, msg, timeout, error
		}

	case StateFastSyncMode:
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
			if peerStats.PendingRequest == -1 {
				msg = createBlockRequestMessage(state, event.PeerID, peerStats.Height)
				if msg != nil {
					peerStats.PendingRequest = msg.Height
					state.PendingRequestsNum++
					timeout = ResponseTimeoutTrigger{msg.PeerID, msg.Height, PeerTimeout}
				} else if state.PendingRequestsNum == 0 {
					timeout = TerminationTimeoutTrigger{state.Height, TerminationTimeout}
				}
			}
			state.PeerMap[event.PeerID] = peerStats
			return state, msg, timeout, error

		case EventRemovePeer:
			if _, ok := state.PeerMap[event.PeerID]; ok {
				pendingRequest = state.PeerMap[event.PeerID].PendingRequest
				if pendingRequest != -1 {
					add(state.FailedRequests, pendingRequest)
				}
				state.PendingRequestsNum--
				delete(state.PeerMap, event.PeerID)
				if state.PeerMap.isEmpty() || state.PendingRequestsNum == 0 {
					timeout = TerminationTimeoutTrigger{state.Height, TerminationTimeout}
				}
			} else {
				error = "Removing unknown peer!"
			}
			return state, msg, timeout, error

		case EventNewBlock:
			if state.PeerMap[event.PeerID] {
				peerStats = state.PeerMap[event.PeerID]
				if peerStats.PendingRequest == event.Height {
					peerStats.PendingRequest = -1
					state.PendingRequestsNum--
					if event.CurrentHeight > peerStats.Height {
						peerStats.Height = event.CurrentHeight
					}
					state.Store[event.Height] = BlockInfo{event.Block, event.Commit, event.PeerID}
					state = verifyBlocks(state) // it can lead to event.PeerID being removed from peer list

					if _, ok := state.PeerMap[event.PeerID]; ok {
						msg = createBlockRequestMessage(state, event.PeerID, peerStats.Height)
						if msg != nil {
							peerStats.PendingRequests = msg.Height
							state.PendingRequestsNum++
							timeout = ResponseTimeoutTrigger{msg.PeerID, msg.Height, PeerTimeout}
						} else if state.PendingRequestsNum == 0 {
							timeout = TerminationTimeoutTrigger{state.Height, TerminationTimeout}
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
				if peerStats.PendingRequest == event.Height {
					add(state.FailedRequests, pendingRequest)
					delete(state.PeerMap, event.PeerID)
					state.PendingRequestsNum--
					if state.PeerMap.isEmpty() || state.PendingRequestsNum == 0 {
						timeout = TimeoutTrigger{state.Height, TerminationTimeout}
					}
				}
			}

			return state, msg, timeout, error

		case EventTerminationTimeout:
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
	blockNumber = -1

	// exist request r in state.FailedRequests such that r <= peerHeight
	if true {
		blockNumber = r
		delete(state.FailedRequests, r)
	} else if state.MaxRequestPending < peerHeight {
		state.MaxRequestPending++
		blockNumber = state.MaxRequestPending // increment state.MaxRequestPending and then return the new value
	}

	if blockNumber > -1 {
		msg = BlockRequestMessage{blockNumber, peerID}
	}
	return msg
}

func verifyBlocks(state State) State {
	done = false
	for !done {
		block = state.Store[height]

		if block != nil {
			// TODO: verify block.Block using block.Commit
			verified = true

			if verified {
				block.Execute()
				state.Height++
			} else {
				add(state.FailedRequests, height)
				if _, ok := state.PeerMap[block.PeerID]; ok {
					pendingRequest = state.PeerMap[block.PeerID].PendingRequest
					if pendingRequest != -1 {
						add(state.FailedRequests, pendingRequest)
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
