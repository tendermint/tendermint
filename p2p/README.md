# `tendermint/tendermint/p2p`

[![CircleCI](https://circleci.com/gh/tendermint/tendermint/p2p.svg?style=svg)](https://circleci.com/gh/tendermint/tendermint/p2p)

`tendermint/tendermint/p2p` provides an abstraction around peer-to-peer communication.<br/>

## MConnection

`MConnection` is a multiplex connection:

__multiplex__ *noun* a system or signal involving simultaneous transmission of
several messages along a single channel of communication.

Each `MConnection` handles message transmission on multiple abstract communication
`Channel`s.  Each channel has a globally unique byte id.
The byte id and the relative priorities of each `Channel` are configured upon
initialization of the connection.

The `MConnection` supports three packet types: Ping, Pong, and Msg.

### Ping and Pong

The ping and pong messages consist of writing a single byte to the connection; 0x1 and 0x2, respectively

When we haven't received any messages on an `MConnection` in a time `pingTimeout`, we send a ping message.
When a ping is received on the `MConnection`, a pong is sent in response.

If a pong is not received in sufficient time, the peer's score should be decremented (TODO).

### Msg

Messages in channels are chopped into smaller msgPackets for multiplexing.

```
type msgPacket struct {
	ChannelID byte
	EOF       byte // 1 means message ends here.
	Bytes     []byte
}
```

The msgPacket is serialized using go-wire, and prefixed with a 0x3.
The received `Bytes` of a sequential set of packets are appended together
until a packet with `EOF=1` is received, at which point the complete serialized message 
is returned for processing by the corresponding channels `onReceive` function.

### Multiplexing

Messages are sent from a single `sendRoutine`, which loops over a select statement that results in the sending
of a ping, a pong, or a batch of data messages. The batch of data messages may include messages from multiple channels.
Message bytes are queued for sending in their respective channel, with each channel holding one unsent message at a time.
Messages are chosen for a batch one a time from the channel with the lowest ratio of recently sent bytes to channel priority.

## Sending Messages

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

## Peer

Each peer has one `MConnection` instance, and includes other information such as whether the connection
was outbound, whether the connection should be recreated if it closes, various identity information about the node, 
and other higher level thread-safe data used by the reactors.

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
