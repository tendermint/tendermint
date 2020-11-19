package p2p

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
)

var _ Reactor = (*ReactorShim)(nil)

type (
	CodecShim interface {
		Marshal(proto.Message) ([]byte, error)
		Unmarshal([]byte) (proto.Message, error)
		Validate(proto.Message) error
	}

	ChannelShim struct {
		Descriptor *ChannelDescriptor
		Channel    *Channel
		InCh       chan Envelope
		OutCh      chan Envelope
		PeerErrCh  chan PeerError
	}

	ReactorShim struct {
		BaseReactor

		Name         string
		Codec        CodecShim
		PeerUpdateCh chan PeerUpdate
		Channels     map[ChannelID]*ChannelShim
	}
)

func NewShim(name string, codec CodecShim, impl Reactor, descriptors []*ChannelDescriptor) *ReactorShim {
	br := *NewBaseReactor(name, impl)

	channels := make(map[ChannelID]*ChannelShim)
	for _, cd := range descriptors {
		cID := ChannelID(cd.ID)
		inCh := make(chan Envelope)
		outCh := make(chan Envelope)
		peerErrCh := make(chan PeerError)

		channels[cID] = &ChannelShim{
			Descriptor: cd,
			Channel:    NewChannel(cID, nil, inCh, outCh, peerErrCh),
			InCh:       inCh,
			OutCh:      outCh,
			PeerErrCh:  peerErrCh,
		}
	}

	return &ReactorShim{
		BaseReactor:  br,
		Name:         name,
		Codec:        codec,
		PeerUpdateCh: make(chan PeerUpdate),
		Channels:     channels,
	}
}

// proxyPeerErrors iterates over each p2p Channel and starts a separate go-routine
// where we listen for peer errors on the channel's PeerErrCh and send a PeerUpdate
// on the PeerUpdateCh with a status PeerStatusBanned.
func (rs *ReactorShim) proxyPeerErrors() {
	for _, c := range rs.Channels {
		go func(peerErrCh chan PeerError) {
			for peerErr := range peerErrCh {
				select {
				case rs.PeerUpdateCh <- PeerUpdate{PeerID: peerErr.PeerID, Status: PeerStatusBanned}:
					rs.Logger.Debug(
						"sent peer update due to error",
						"reactor", rs.Name, "peer", peerErr.PeerID.String(), "status", PeerStatusBanned, "err", peerErr.Err,
					)

				default:
					rs.Logger.Debug(
						"dropped peer update due to error",
						"reactor", rs.Name, "peer", peerErr.PeerID.String(), "status", PeerStatusBanned, "err", peerErr.Err,
					)
				}
			}
		}(c.PeerErrCh)
	}
}

// proxyPeerEnvelopes iterates over each p2p Channel and starts a separate
// go-routine where we listen for outbound envelopes sent during Receive
// executions and proxy them to the coressponding Peer using the To field from
// the envelope.
func (rs *ReactorShim) proxyPeerEnvelopes() {
	for _, c := range rs.Channels {
		go func(chID byte, outCh chan Envelope) {
			for e := range outCh {
				src := rs.Switch.peers.Get(ID(e.To.String()))
				if src == nil {
					panic(fmt.Sprintf("failed to proxy envelope; failed to find peer (%s)", e.To))
				}

				bz, err := rs.Codec.Marshal(e.Message)
				if err != nil {
					panic(fmt.Sprintf("failed to proxy envelope; failed to encode message: %s", err))
				}

				_ = src.Send(chID, bz)
			}
		}(c.Descriptor.ID, c.OutCh)
	}
}

func (rs *ReactorShim) GetChannels() []*ChannelDescriptor {
	descriptors := make([]*ChannelDescriptor, len(rs.Channels))
	i := 0

	for _, c := range rs.Channels {
		descriptors[i] = c.Descriptor
		i++
	}

	return descriptors
}

// OnStart executes the reactor shim's OnStart hook where we start all the
// necessary go-routines in order to proxy peer errors and messages.
func (rs *ReactorShim) OnStart() error {
	if rs.IsRunning() {
		rs.proxyPeerErrors()
		rs.proxyPeerEnvelopes()
	}

	return nil
}

// OnStop executes the reactor shim's OnStop hook where all p2p Channels are
// closed and the PeerUpdateCh is closed.
func (rs *ReactorShim) OnStop() {
	for _, c := range rs.Channels {
		if err := c.Channel.Close(); err != nil {
			rs.Logger.Error("failed to close channel", "reactor", rs.Name, "ch_id", c.Channel.ID, "err", err)
		}
	}

	close(rs.PeerUpdateCh)
}

// AddPeer sends a PeerUpdate with status PeerStatusUp on the PeerUpdateCh.
// The embedding reactor must be sure to listen for messages on this channel to
// handle adding a peer.
func (rs *ReactorShim) AddPeer(peer Peer) {
	peerID, err := PeerIDFromString(string(peer.ID()))
	if err != nil {
		// It is OK to panic here as we'll be removing the Reactor interface and
		// Peer type in favor of using a PeerID directly.
		panic(err)
	}

	select {
	case rs.PeerUpdateCh <- PeerUpdate{PeerID: peerID, Status: PeerStatusUp}:
		rs.Logger.Debug("sent peer update", "reactor", rs.Name, "peer", peerID.String(), "status", PeerStatusUp)

	default:
		rs.Logger.Debug("dropped peer update", "reactor", rs.Name, "peer", peerID.String(), "status", PeerStatusUp)
	}
}

// RemovePeer sends a PeerUpdate with status PeerStatusDown on the PeerUpdateCh.
// The embedding reactor must be sure to listen for messages on this channel to
// handle removing a peer.
func (rs *ReactorShim) RemovePeer(peer Peer, reason interface{}) {
	peerID, err := PeerIDFromString(string(peer.ID()))
	if err != nil {
		// It is OK to panic here as we'll be removing the Reactor interface and
		// Peer type in favor of using a PeerID directly.
		panic(err)
	}

	select {
	case rs.PeerUpdateCh <- PeerUpdate{PeerID: peerID, Status: PeerStatusDown}:
		rs.Logger.Debug("sent peer update", "reactor", rs.Name, "peer", peerID.String(), "status", PeerStatusDown)

	default:
		rs.Logger.Debug("dropped peer update", "reactor", rs.Name, "peer", peerID.String(), "status", PeerStatusDown)
	}
}

func (rs *ReactorShim) Receive(chID byte, src Peer, msgBytes []byte) {
	if !rs.IsRunning() {
		return
	}

	cID := ChannelID(chID)
	channel, ok := rs.Channels[cID]
	if !ok {
		rs.Logger.Error("unexpected channel", "peer", src, "ch_id", chID)
		return
	}

	msg, err := rs.Codec.Unmarshal(msgBytes)
	if err != nil {
		rs.Logger.Error("error decoding message", "peer", src, "ch_id", cID, "msg", msg, "err", err)
		// TODO: We need a way to handle custom business logic on error.
		return
	}

	if err := rs.Codec.Validate(msg); err != nil {
		rs.Logger.Error("invalid message", "peer", src, "ch_id", cID, "msg", msg, "err", err)
		// TODO: We need a way to handle custom business logic on error.
		return
	}

	peerID, err := PeerIDFromString(string(src.ID()))
	if err != nil {
		// It is OK to panic here as we'll be removing the Reactor interface and
		// Peer type in favor of using a PeerID directly.
		panic(err)
	}

	select {
	case channel.InCh <- Envelope{From: peerID, Message: msg}:
		rs.Logger.Debug("proxied envelope", "reactor", rs.Name, "ch_id", cID, "peer", peerID.String())

	default:
		rs.Logger.Debug("dropped envelope", "reactor", rs.Name, "ch_id", cID, "peer", peerID.String())
	}
}
