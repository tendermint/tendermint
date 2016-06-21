# `tendermint/go-p2p`

[![CircleCI](https://circleci.com/gh/tendermint/go-p2p.svg?style=svg)](https://circleci.com/gh/tendermint/go-p2p)

`tendermint/go-p2p` provides an abstraction around peer-to-peer communication.<br/>

## Peer/MConnection/Channel

Each peer has one `MConnection` (multiplex connection) instance.

__multiplex__ *noun* a system or signal involving simultaneous transmission of
several messages along a single channel of communication.

Each `MConnection` handles message transmission on multiple abstract communication
`Channel`s.  Each channel has a globally unique byte id.
The byte id and the relative priorities of each `Channel` are configured upon
initialization of the connection.

There are two methods for sending messages:
```go
func (m MConnection) Send(chID byte, msg interface{}) bool {}
func (m MConnection) TrySend(chID byte, msg interface{}) bool {}
```

`Send(chID, msg)` is a blocking call that waits until `msg` is successfully queued
for the channel with the given id byte `chID`.  The message `msg` is serialized
using the `tendermint/wire` submodule's `WriteBinary()` reflection routine.

`TrySend(chID, msg)` is a nonblocking call that returns false if the channel's
queue is full.

`Send()` and `TrySend()` are also exposed for each `Peer`.

## Switch/Reactor

The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.

```go
// Declare a MyReactor reactor that handles messages on MyChannelID.
type MyReactor struct{}

func (reactor MyReactor) GetChannels() []*ChannelDescriptor {
    return []*ChannelDescriptor{ChannelDescriptor{ID:MyChannelID, Priority: 1}}
}

func (reactor MyReactor) Receive(chID byte, peer *Peer, msgBytes []byte) {
    r, n, err := bytes.NewBuffer(msgBytes), new(int64), new(error)
    msgString := ReadString(r, n, err)
    fmt.Println(msgString)
}

// Other Reactor methods omitted for brevity
...

switch := NewSwitch([]Reactor{MyReactor{}})

...

// Send a random message to all outbound connections
for _, peer := range switch.Peers().List() {
    if peer.IsOutbound() {
        peer.Send(MyChannelID, "Here's a random message")
    }
}
```

### PexReactor/AddrBook

A `PEXReactor` reactor implementation is provided to automate peer discovery.

```go
book := p2p.NewAddrBook(addrBookFilePath)
pexReactor := p2p.NewPEXReactor(book)
...
switch := NewSwitch([]Reactor{pexReactor, myReactor, ...})
```
