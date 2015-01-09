# `tendermint/p2p`

`tendermint/p2p` provides an abstraction around peer-to-peer communication.<br/>

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
func (m MConnection) Send(chId byte, msg interface{}) bool {}
func (m MConnection) TrySend(chId byte, msg interface{}) bool {}
```

`Send(chId, msg)` is a blocking call that waits until `msg` is successfully queued
for the channel with the given id byte `chId`.  The message `msg` is serialized
using the `tendermint/binary` submodule's `WriteBinary()` reflection routine.

`TrySend(chId, msg)` is a nonblocking call that returns false if the channel's
queue is full.

`Send()` and `TrySend()` are also exposed for each `Peer`.

## Switch/Reactor

The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.

```go
// Declare a MyReactor reactor that handles messages on MyChannelId.
type MyReactor struct{}

func (reactor MyReactor) GetChannels() []*ChannelDescriptor {
    return []*ChannelDescriptor{ChannelDescriptor{Id:MyChannelId, Priority: 1}}
}

func (reactor MyReactor) Receive(chId byte, peer *Peer, msgBytes []byte) {
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
        peer.Send(MyChannelId, "Here's a random message")
    }
}
```

### PexReactor/AddrBook

A `PEXReactor` reactor implementation is provided to automate peer discovery.

```go
book := p2p.NewAddrBook(config.App.GetString("AddrBookFile"))
pexReactor := p2p.NewPEXReactor(book)
...
switch := NewSwitch([]Reactor{pexReactor, myReactor, ...})
```
