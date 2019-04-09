# ADR 037: Peer Behaviour Interface

## Changelog
* 07-03-2019: Initial draft

## Context

The responsibility for signaling and acting upon peer behaviour lacks a single 
owning component and is heavily coupled with the network stack, (cf.
[#2067](https://github.com/tendermint/tendermint/issues/2067)). Reactors
maintain a reference to the `p2p.Switch` which they use to call 
`switch.StopPeerForError(...)` when a peer misbehaves and 
`switch.MarkAsGood(...)` when a peer contributes in some meaningful way. 
While the switch handles `StopPeerForError` internally, the `MarkAsGood` 
method delegates to another component, `p2p.AddrBook`. This scheme of delegation 
across Switch obscures the responsibility for handling peer behaviour
and ties up the reactors in a larger dependency graph when testing.

## Decision

Introduce a PeerBehaviour interface and concrete implementations which
provide methods for reactors to signal peer behaviour without direct
coupling `p2p.Switch`.  Introduce a ErrPeer to provide 
concrete reasons for stopping peers.

### Implementation Changes

PeerBehaviour then becomes an interface for signaling peer errors as well
as marking peers for `good`.

XXX: It might be better to pass p2p.ID instead of the whole peer but as
a first draft maintain the underlying implementation as much as
possible.

```go
type PeerBehaviour interface {
    Errored(peer Peer, reason ErrPeer)
    MarkPeerAsGood(peer Peer)
}
```

Instead of signaling peers to stop with arbitrary reasons:
`reason interface{}` 

We introduce a concrete error type ErrPeer:
```go
type ErrPeer int

const (
    ErrPeerUnknown = iota
    ErrPeerBadMessage
    ErrPeerMessageOutofOrder
    ...
)
```

As a first iteration we provide a concrete implementation which wraps
the switch:
```go
type SwitchedPeerBehaviour struct {
    sw *Switch
}

func (spb *SwitchedPeerBehaviour) Errored(peer Peer, reason ErrPeer) {
    spb.sw.StopPeerForError(peer, reason)
}

func (spb *SwitchedPeerBehaviour) MarkPeerAsGood(peer Peer) {
    spb.sw.MarkPeerAsGood(peer)
}

func NewSwitchedPeerBehaviour(sw *Switch) *SwitchedPeerBehaviour {
    return &SwitchedPeerBehaviour{
        sw: sw,
    }
}
```

Reactors, which are often difficult to unit test (Cf. [#3506](https://github.com/tendermint/tendermint/pull/3506)). could use an implementation which exposes the signals produced by the reactor in
manufactured scenarios:

```go
type PeerErrors map[Peer][]ErrPeer
type GoodPeers map[Peer]bool

type StorePeerBehaviour struct {
    pe PeerErrors
    gp GoodPeers
}

func NewStorePeerBehaviour() *StorePeerBehaviour{
    return &StorePeerBehaviour{
        pe: make(PeerErrors),
        gp: GoodPeers{},
    }
}

func (spb StorePeerBehaviour) Errored(peer Peer, reason ErrPeer) {
    if _, ok := spb.pe[peer]; !ok {
        spb.pe[peer] = []ErrPeer{reason}
    } else {
        spb.pe[peer] = append(spb.pe[peer], reason)
    }
}

func (mpb *StorePeerBehaviour) GetPeerErrors() PeerErrors {
    return mpb.pe
}

func (spb *StorePeerBehaviour) MarkPeerAsGood(peer Peer) {
    if _, ok := spb.gp[peer]; !ok {
        spb.gp[peer] = true
    }
}

func (spb *StorePeerBehaviour) GetGoodPeers() GoodPeers {
    return spb.gp
}
```

## Status

Proposed

## Consequences

### Positive

    * De-couple signaling from acting upon peer behaviour.
    * Reduce the coupling of reactors and the Switch and the network
      stack
    * The responsibility of managing peer behaviour can be migrated to
      a single component instead of split between the switch and the
      address book.

### Negative

    * The first iteration will simply wrap the Switch and introduce a
      level of indirection.

### Neutral

## References

* [#2067](https://github.com/tendermint/tendermint/issues/2067): P2P Refactor
* [#3506](https://github.com/tendermint/tendermint/pull/3506): ADR 036: Blockchain Reactor Refactor
