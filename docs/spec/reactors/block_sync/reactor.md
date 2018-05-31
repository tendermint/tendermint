# Blockchain Reactor

The Blockchain Reactor's high level responsibility is to enable peers who are
far behind the current state of the consensus to quickly catch up by downloading
many blocks in parallel, verifying their commits, and executing them against the
ABCI application.

Tendermint full nodes run the Blockchain Reactor as a service to provide blocks
to new nodes. New nodes run the Blockchain Reactor in "fast_sync" mode,
where they actively make requests for more blocks until they sync up.
Once caught up, "fast_sync" mode is disabled and the node switches to
using (and turns on) the Consensus Reactor.

## Message Types

```go
const (
    msgTypeBlockRequest    = byte(0x10)
    msgTypeBlockResponse   = byte(0x11)
    msgTypeNoBlockResponse = byte(0x12)
    msgTypeStatusResponse  = byte(0x20)
    msgTypeStatusRequest   = byte(0x21)
)
```

```go
type bcBlockRequestMessage struct {
    Height int64
}

type bcNoBlockResponseMessage struct {
    Height int64
}

type bcBlockResponseMessage struct {
    Block Block
}

type bcStatusRequestMessage struct {
    Height int64

type bcStatusResponseMessage struct {
    Height int64
}
```

## Protocol

TODO

## Channels

Defines `maxMsgSize` for the maximum size of incoming messages,
`SendQueueCapacity` and `RecvBufferCapacity` for maximum sending and
receiving buffers respectively. These are supposed to prevent amplification
attacks by setting up the upper limit on how much data we can receive & send to
a peer.

Sending incorrectly encoded data will result in stopping the peer.
