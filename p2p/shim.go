package p2p

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gogo/protobuf/proto"
)

// ============================================================================
// TODO: Types and business logic below are temporary and will be removed once
// the legacy p2p stack is removed in favor of the new model.
//
// ref: https://github.com/tendermint/tendermint/issues/5670
// ============================================================================

var _ Reactor = (*ReactorShim)(nil)

type (
	messageValidator interface {
		Validate() error
	}

	// ReactorShim defines a generic shim wrapper around a BaseReactor. It is
	// responsible for wiring up legacy p2p behavior to the new p2p semantics
	// (e.g. proxying Envelope messages to legacy peers).
	ReactorShim struct {
		BaseReactor

		Name         string
		PeerUpdateCh chan PeerUpdate
		Channels     map[ChannelID]*ChannelShim
	}

	// ChannelShim defines a generic shim wrapper around a legacy p2p channel
	// and the new p2p Channel. It also includes the raw bi-directional Go channels
	// so we can proxy message delivery.
	ChannelShim struct {
		Descriptor *ChannelDescriptor
		Channel    *Channel
		InCh       chan Envelope
		OutCh      chan Envelope
		PeerErrCh  chan PeerError
	}

	// ChannelDescriptorShim defines a shim wrapper around a legacy p2p channel
	// and the proto.Message the new p2p Channel is responsible for handling.
	// A ChannelDescriptorShim is not contained in ReactorShim, but is rather
	// used to construct a ReactorShim.
	ChannelDescriptorShim struct {
		MsgType    proto.Message
		Descriptor *ChannelDescriptor
	}
)

func NewReactorShim(name string, descriptors map[ChannelID]*ChannelDescriptorShim) *ReactorShim {
	channels := make(map[ChannelID]*ChannelShim)

	for _, cds := range descriptors {
		chShim := NewChannelShim(cds, 1)
		channels[chShim.Channel.ID] = chShim
	}

	rs := &ReactorShim{
		Name:         name,
		PeerUpdateCh: make(chan PeerUpdate),
		Channels:     channels,
	}

	rs.BaseReactor = *NewBaseReactor(name, rs)

	return rs
}

func NewChannelShim(cds *ChannelDescriptorShim, buf uint) *ChannelShim {
	inCh := make(chan Envelope, buf)
	outCh := make(chan Envelope, buf)
	peerErrCh := make(chan PeerError, buf)

	return &ChannelShim{
		Descriptor: cds.Descriptor,
		Channel:    NewChannel(ChannelID(cds.Descriptor.ID), cds.MsgType, inCh, outCh, peerErrCh),
		InCh:       inCh,
		OutCh:      outCh,
		PeerErrCh:  peerErrCh,
	}
}

// proxyPeerEnvelopes iterates over each p2p Channel and starts a separate
// go-routine where we listen for outbound envelopes sent during Receive
// executions (or anything else that may send on the Channel) and proxy them to
// the corresponding Peer using the To field from the envelope.
func (rs *ReactorShim) proxyPeerEnvelopes() {
	for _, cs := range rs.Channels {
		go func(cs *ChannelShim) {
			for e := range cs.OutCh {
				msg := proto.Clone(cs.Channel.messageType)
				msg.Reset()

				wrapper, ok := msg.(Wrapper)
				if ok {
					if err := wrapper.Wrap(e.Message); err != nil {
						rs.Logger.Error(
							"failed to proxy envelope; failed to wrap message",
							"ch_id", cs.Descriptor.ID,
							"msg", e.Message,
							"err", err,
						)
						continue
					}
				} else {
					msg = e.Message
				}

				bz, err := proto.Marshal(msg)
				if err != nil {
					rs.Logger.Error(
						"failed to proxy envelope; failed to encode message",
						"ch_id", cs.Descriptor.ID,
						"msg", e.Message,
						"err", err,
					)
					continue
				}

				if e.Broadcast {
					rs.Switch.Broadcast(cs.Descriptor.ID, bz)
				} else {
					src := rs.Switch.peers.Get(ID(e.To.String()))
					if src == nil {
						rs.Logger.Error(
							"failed to proxy envelope; failed to find peer ",
							"ch_id", cs.Descriptor.ID,
							"msg", e.Message,
							"peer", e.To.String(),
						)
						continue
					}

					if !src.Send(cs.Descriptor.ID, bz) {
						rs.Logger.Error(
							"failed to proxy message to peer",
							"ch_id", cs.Descriptor.ID,
							"msg", e.Message,
							"peer", e.To.String(),
						)
					}
				}
			}
		}(cs)
	}
}

// handlePeerErrors iterates over each p2p Channel and starts a separate go-routine
// where we listen for peer errors. For each peer error, we find the peer from
// the legacy p2p Switch and execute a StopPeerForError call with the corresponding
// peer error.
func (rs *ReactorShim) handlePeerErrors() {
	for _, cs := range rs.Channels {
		go func(cs *ChannelShim) {
			for pErr := range cs.PeerErrCh {
				if !pErr.PeerID.Empty() {
					peer := rs.Switch.peers.Get(ID(pErr.PeerID.String()))
					if peer == nil {
						panic(fmt.Sprintf("failed to handle peer error; failed to find peer (%s)", pErr.PeerID))
					}

					rs.Switch.StopPeerForError(peer, pErr.Err)
				}
			}
		}(cs)
	}
}

// OnStart executes the reactor shim's OnStart hook where we start all the
// necessary go-routines in order to proxy peer envelopes and errors per p2p
// Channel.
func (rs *ReactorShim) OnStart() error {
	if rs.Switch == nil {
		return errors.New("proxyPeerEnvelopes: reactor shim switch is nil")
	}

	// start envelope proxying and peer error handling in separate go routines
	rs.proxyPeerEnvelopes()
	rs.handlePeerErrors()

	return nil
}

// OnStop executes the reactor shim's OnStop hook where all inbound p2p Channels
// are closed and the PeerUpdateCh is closed. The caller must be sure to also
// stop the real reactor so the shim's go routines can exit successfully.
func (rs *ReactorShim) OnStop() {
	for _, cs := range rs.Channels {
		close(cs.InCh)
	}

	close(rs.PeerUpdateCh)
}

// GetChannel returns a p2p Channel reference for a given ChannelID. If no
// Channel exists, nil is returned.
func (rs *ReactorShim) GetChannel(cID ChannelID) *Channel {
	channelShim, ok := rs.Channels[cID]
	if ok {
		return channelShim.Channel
	}

	return nil
}

// GetChannels implements the legacy Reactor interface for getting a slice of all
// the supported ChannelDescriptors.
func (rs *ReactorShim) GetChannels() []*ChannelDescriptor {
	descriptors := make([]*ChannelDescriptor, len(rs.Channels))

	sortedChIDs := make([]ChannelID, 0, len(descriptors))
	for cID := range rs.Channels {
		sortedChIDs = append(sortedChIDs, cID)
	}

	sort.Slice(sortedChIDs, func(i, j int) bool { return sortedChIDs[i] < sortedChIDs[j] })

	for i, cID := range sortedChIDs {
		descriptors[i] = rs.Channels[cID].Descriptor
	}

	return descriptors
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

	rs.PeerUpdateCh <- PeerUpdate{PeerID: peerID, Status: PeerStatusUp}
	rs.Logger.Debug("sent peer update", "reactor", rs.Name, "peer", peerID.String(), "status", PeerStatusUp)
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

	rs.PeerUpdateCh <- PeerUpdate{PeerID: peerID, Status: PeerStatusDown}
	rs.Logger.Debug(
		"sent peer update",
		"reactor", rs.Name,
		"peer", peerID.String(),
		"reason", reason,
		"status", PeerStatusDown,
	)
}

// Receive implements a generic wrapper around implementing the Receive method
// on the legacy Reactor p2p interface. If the reactor is running, Receive will
// find the corresponding new p2p Channel, create and decode the appropriate
// proto.Message from the msgBytes, execute any validation and finally construct
// and send a p2p Envelope on the appropriate p2p Channel.
func (rs *ReactorShim) Receive(chID byte, src Peer, msgBytes []byte) {
	if !rs.IsRunning() {
		return
	}

	cID := ChannelID(chID)
	channelShim, ok := rs.Channels[cID]
	if !ok {
		rs.Logger.Error("unexpected channel", "peer", src, "ch_id", chID)
		return
	}

	peerID, err := PeerIDFromString(string(src.ID()))
	if err != nil {
		// It is OK to panic here as we'll be removing the Reactor interface and
		// Peer type in favor of using a PeerID directly.
		panic(err)
	}

	msg := proto.Clone(channelShim.Channel.messageType)
	msg.Reset()

	if err := proto.Unmarshal(msgBytes, msg); err != nil {
		rs.Logger.Error("error decoding message", "peer", src, "ch_id", cID, "msg", msg, "err", err)
		rs.Switch.StopPeerForError(src, err)
		return
	}

	validator, ok := msg.(messageValidator)
	if ok {
		if err := validator.Validate(); err != nil {
			rs.Logger.Error("invalid message", "peer", src, "ch_id", cID, "msg", msg, "err", err)
			rs.Switch.StopPeerForError(src, err)
			return
		}
	}

	wrapper, ok := msg.(Wrapper)
	if ok {
		var err error

		msg, err = wrapper.Unwrap()
		if err != nil {
			rs.Logger.Error("failed to unwrap message", "peer", src, "ch_id", chID, "msg", msg, "err", err)
			return
		}
	}

	go func() {
		channelShim.InCh <- Envelope{From: peerID, Message: msg}
		rs.Logger.Debug("proxied envelope", "reactor", rs.Name, "ch_id", cID, "peer", peerID.String())
	}()
}
