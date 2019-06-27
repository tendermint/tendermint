# ADR 042: Blockhchain Reactor Riri-Org

## Changelog
* 18-06-2019: Initial draft

## Context

The blockchain reactor is responsible for two high level processes:sending/receiving blocks from peers and FastSync-ing blocks to catch upnode who is far behind.  The goal of [ADR-40](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-040-blockchain-reactor-refactor.md) was to refactor these two processes by separating business logic currently wrapped up in go-channels into pure `handle*` functions.  While the ADR specified what the final form of the reactor might look like it lacked guidance on intermediary steps to get there. 
The following diagram illustrates the state of the [blockchain-reorg](https://github.com/tendermint/tendermint/pull/35610) reactor which will be refered to as `v1`.

![v1 Blockchain Reactor Architecture
Diagram](https://github.com/tendermint/tendermint/blob/f9e556481654a24aeb689bdadaf5eab3ccd66829/docs/architecture/img/blockchain-reactor-v1.png)

While `v1` of the blockchain reactor has shown significant improvements in terms of simplifying the concurrency model, the current PR has run into few roadblocks.

* The current PR large and difficult to review.
* Block gossiping and fast sync processes are highly coupled to the shared `Pool` data structure.
* Peer communication is spread over multiple components creating complex dependency graph which must be mocked out during testing.
* Timeouts modeled as stateful tickers introduce non-determinism in tests

This ADR is meant to specify the missing components and control nessesary to acheive [ADR-40](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-040-blockchain-reactor-refactor.md).

## Decision

Partition the responsibilities of the blockchain reactor into a set of components which communicate exclusively with events. Events will contain timestamps allowing each component to track time as internal state. The internal state will be mutated by a set of `handle*` which will produce event(s). The integration between components will happen in the reactor and reactor tests will then become integration tests between components. This design will be known as `v2`.

![v2 Blockchain Reactor Architecture
Diagram](https://github.com/tendermint/tendermint/blob/f9e556481654a24aeb689bdadaf5eab3ccd66829/docs/architecture/img/blockchain-reactor-v2.png)

### Reactor changes in detail

The reactor will include a demultiplexing routine which will send each message to each sub routine for independant processing. Each sub routine will then select the messages it's interested in and call the handle specific function specified in [ADR-40](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-040-blockchain-reactor-refactor.md).

```go
func demuxRoutine(msgs, scheduleMsgs, processorMsgs, ioMsgs) {
    timer := time.NewTicker(interval)
    for {
        select {
            case <-timer.C:
                now := evTimeCheck{time.Now()}
                schedulerMsgs <- now
                processorMsgs <- now
                ioMsgs <- now
            case msg:= <- msgs:
                schedulerMsgs <- msg
                processorMsgs <- msg
                ioMesgs <- msg
                if msg == stop {
                    break;
                }
        }
    }
}

func processRoutine(input chan Message, output chan Message) {
    processor := NewProcessor(..)
    for {
        msg := <- input
        switch msg := msg.(type) {
            case bcBlockRequestMessage:
                output <- processor.handleBlockRequest(msg, time.Now())
            ...
            case stop:
                processor.stop()
                break;
    }
}

func scheduleRoutine(input chan Message, output chan Message) {
    schelduer = NewScheduler(...)
    for {
        msg := <-msgs
        switch msg := input.(type) {
            case bcBlockResponseMessage:
                output <- scheduler.handleBlockResponse(msg, time.Now())
            ...
            case stop:
                schedule.stop()
                break;
        }
    }
}
```

## Lifecycle management

A set of routines for individual processes allow processes to run in parallel with clear lifecycle management. `Start`, `Stop`, and `AddPeer` hooks currently present in the reactor will delegate to the sub-routines allowing them to manage internal state independant without further coupling to the reactor.

```go
func (r *BlockChainReactor) Start() {
    r.msgs := make(chan Message, maxInFlight)
    schedulerMsgs := make(chan Message)
    processorMsgs := make(chan Message)
    ioMsgs := make(chan Message)

    go processorRoutine(processorMsgs, r.msgs)
    go scheduleRoutine(schedulerMsgs, r.msgs)
    go ioRoutine(ioMsgs, r.msgs)
    ...
}

...
func (r *BlockchainReactor) Stop() {
    ...
    r.msgs <- stop
    ...
}
...

func (r *BlockchainReactor) AddPeer(peer p2p.Peer) {
    ...
    r.msgs <- bcAddPeerEv{peer.ID}
    ...
}

```

## IO handling
An io handling routine within the reactor will isolate peer communication.

```go
func (r *BlockchainReacor) ioRoutine(chan ioMsgs, ...) {
    ...
    for {
        msg := <-ioMsgs
        switch msg := msg.(type) {
            case scBlockRequestMessage:
                r.sendBlockToPeer(...)
            case scStatusRequestMessage
                r.sendStatusResponseToPeer(...)
            case bcPeerError
                r.Swtich.StopPeerForError(msg.src)
                ...
            ...
            case bcFinished
                break;
        }
    }
}

```
### Processor internals 

The processor will be responsible for validating and processing blocks.
The Processor will maintain an internal cursor `height` of the last
processed block. As a set of unordered blocks arrive, the the Processor
will check if it has `height+1` nessary to process the next block.

```go
type Proccesor struct {
    height ...
    state ...
    blocks [height]*Block
    peers[height]PeerID
    lastTouch timestamp
}

func (proc *Processor) handleBlockResponse(peerID, block, time) {
    lastTouch = time
    if block.height < height {
        // skip
    } else if blocks[block.height] {
        return errDuplicateBlock{}
    } else  {
        blocks[block.height] = block
    }

    if blocks[height] && blocks[height+1] {
        ... = processBlock(blocks[height], blocks[height+1])
         bcR.store.SaveBlock(first, firstParts, second.LastCommit)
        ... = proc.state.Validators.VerifyCommit
        delete blocks[height]
        height++
    }
    return pcBlockProcessed{height}
}

func (proc *Processor) handleRemovePeer(peerID) {
    events = []
    // Delete all unprocessed blocks from peerID
    for i = height; i < len(blocks); i++ {
        if peers[i] == peerID {
            events = append(events, pcBlockReschedule{height})
            delete block[height]
        }
    }
    return events
}

func handleTimeCheckEv(time) {
    if time - lastTouch > timeout {
        // Timeout the processor
        ...
    }
}
```

## Schedule
The scheduler (previously the pool) is responsible for squeding peer status requests and block request responses.

```go
type schedule {
    ...
}

type Scheduler struct {
    ...
    schedule schedule{}
}

func addPeer(peerID) {
	schedule.addPeer(peerID)
}

func handleStatusResponse(peerID, height, time) {
	schedule.touchPeer(peerID, time)
	schedule.setPeerHeight(peerID, height)
}

func handleBlockResponseMessage(peerID, height, block, time) {
	schedule.touchPeer(peerID, time)
	schedule.markReceived(peerID, height, size(block))
}

func handleNoBlockResponseMessage(peerID, height, time) {
	schedule.touchPeer(peerID, time)
    // reschedule that block, punish peer...
    ...
}

func handleTimeCheckEv(time) {
	// clean peer list

    events = []
	for peerID := range schedule.peersTouchedSince(time) {
		pending = schedule.pendingFrom(peerID) 
		schedule.setPeerState(peerID, timedout)
		schedule.resetBlocks(pending)
		events = append(events, peerTimeout{peerID})
    }

	events = append(events, schedule.getSchedule())

    return events
}
```

## Peer
The Peer Stores per peer state based on messages received by the scheduler.

```go
type Peer struct {
	lastTouched timestamp
	lastDownloaded timestamp
	pending map[height]struct{}
	height height // max height for the peer
	state {
		pending,   // we know the peer but not the height
		active,    // we know the height
		timeout    // the peer has timed out
	}
}
```

## Status

Work in progress

## Consequences

### Positive

* Test become deterministic
* Simulation becomes a-termporal: no need wait for a wall-time timeout
* Peer Selection can be independantly tested/simulated
* Develop a general approach to refactoring reactors

### Negative

### Neutral

### Implementaiton Path

* Implement the scheduler, test the scheduler, review the rescheduler
* Implement the processor, test the processor, review the processor
* Implement the demuxer, write integration test, review integraiton tests

## References


* [ADR-40](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-040-blockchain-reactor-refactor.md) was to refactor these two
* [Blockchain re-org](https://github.com/tendermint/tendermint/pull/3561)

