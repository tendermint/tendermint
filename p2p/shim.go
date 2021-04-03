package p2p

import (
	"errors"
	"sort"

	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/log"
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

		Name        string
		PeerUpdates *PeerUpdates
		Channels    map[ChannelID]*ChannelShim
	}

	// ChannelShim defines a generic shim wrapper around a legacy p2p channel
	// and the new p2p Channel. It also includes the raw bi-directional Go channels
	// so we can proxy message delivery.
	ChannelShim struct {
		Descriptor *ChannelDescriptor
		Channel    *Channel
		inCh       chan<- Envelope
		outCh      <-chan Envelope
		errCh      <-chan PeerError
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

func NewReactorShim(logger log.Logger, name string, descriptors map[ChannelID]*ChannelDescriptorShim) *ReactorShim {
	channels := make(map[ChannelID]*ChannelShim)

	for _, cds := range descriptors {
		chShim := NewChannelShim(cds, 0)
		channels[chShim.Channel.ID] = chShim
	}

	rs := &ReactorShim{
		Name:        name,
		PeerUpdates: NewPeerUpdates(make(chan PeerUpdate), 0),
		Channels:    channels,
	}

	rs.BaseReactor = *NewBaseReactor(name, rs)
	rs.SetLogger(logger)

	return rs
}

func NewChannelShim(cds *ChannelDescriptorShim, buf uint) *ChannelShim {
	inCh := make(chan Envelope, buf)
	outCh := make(chan Envelope, buf)
	errCh := make(chan PeerError, buf)
	return &ChannelShim{
		Descriptor: cds.Descriptor,
		Channel: NewChannel(
			ChannelID(cds.Descriptor.ID),
			cds.MsgType,
			inCh,
			outCh,
			errCh,
		),
		inCh:  inCh,
		outCh: outCh,
		errCh: errCh,
	}
}

// proxyPeerEnvelopes iterates over each p2p Channel and starts a separate
// go-routine where we listen for outbound envelopes sent during Receive
// executions (or anything else that may send on the Channel) and proxy them to
// the corresponding Peer using the To field from the envelope.
func (rs *ReactorShim) proxyPeerEnvelopes() {
	for _, cs := range rs.Channels {
		go func(cs *ChannelShim) {
			for e := range cs.outCh {
				msg := proto.Clone(cs.Channel.messageType)
				msg.Reset()

				wrapper, ok := msg.(Wrapper)
				if ok {
					if err := wrapper.Wrap(e.Message); err != nil {
						rs.Logger.Error(
							"failed to proxy envelope; failed to wrap message",
							"ch_id", cs.Descriptor.ID,
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
						"err", err,
					)
					continue
				}

				switch {
				case e.Broadcast:
					rs.Switch.Broadcast(cs.Descriptor.ID, bz)

				case e.To != "":
					src := rs.Switch.peers.Get(e.To)
					if src == nil {
						rs.Logger.Debug(
							"failed to proxy envelope; failed to find peer",
							"ch_id", cs.Descriptor.ID,
							"peer", e.To,
						)
						continue
					}

					if !src.Send(cs.Descriptor.ID, bz) {
						rs.Logger.Error(
							"failed to proxy message to peer",
							"ch_id", cs.Descriptor.ID,
							"peer", e.To,
						)
					}

				default:
					rs.Logger.Error("failed to proxy envelope; missing peer ID", "ch_id", cs.Descriptor.ID)
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
			for pErr := range cs.errCh {
				if pErr.NodeID != "" {
					peer := rs.Switch.peers.Get(pErr.NodeID)
					if peer == nil {
						rs.Logger.Error("failed to handle peer error; failed to find peer", "peer", pErr.NodeID)
						continue
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
	sortedChIDs := make([]ChannelID, 0, len(rs.Channels))
	for cID := range rs.Channels {
		sortedChIDs = append(sortedChIDs, cID)
	}

	sort.Slice(sortedChIDs, func(i, j int) bool { return sortedChIDs[i] < sortedChIDs[j] })

	descriptors := make([]*ChannelDescriptor, len(rs.Channels))
	for i, cID := range sortedChIDs {
		descriptors[i] = rs.Channels[cID].Descriptor
	}

	return descriptors
}

// AddPeer sends a PeerUpdate with status PeerStatusUp on the PeerUpdateCh.
// The embedding reactor must be sure to listen for messages on this channel to
// handle adding a peer.
func (rs *ReactorShim) AddPeer(peer Peer) {
	select {
	case rs.PeerUpdates.reactorUpdatesCh <- PeerUpdate{NodeID: peer.ID(), Status: PeerStatusUp}:
		rs.Logger.Debug("sent peer update", "reactor", rs.Name, "peer", peer.ID(), "status", PeerStatusUp)

	case <-rs.PeerUpdates.Done():
		// NOTE: We explicitly DO NOT close the PeerUpdatesCh's updateCh go channel.
		// This is because there may be numerous spawned goroutines that are
		// attempting to send on the updateCh go channel and when the reactor stops
		// we do not want to preemptively close the channel as that could result in
		// panics sending on a closed channel. This also means that reactors MUST
		// be certain there are NO listeners on the updateCh channel when closing or
		// stopping.
	}
}

// RemovePeer sends a PeerUpdate with status PeerStatusDown on the PeerUpdateCh.
// The embedding reactor must be sure to listen for messages on this channel to
// handle removing a peer.
func (rs *ReactorShim) RemovePeer(peer Peer, reason interface{}) {
	select {
	case rs.PeerUpdates.reactorUpdatesCh <- PeerUpdate{NodeID: peer.ID(), Status: PeerStatusDown}:
		rs.Logger.Debug(
			"sent peer update",
			"reactor", rs.Name,
			"peer", peer.ID(),
			"reason", reason,
			"status", PeerStatusDown,
		)

	case <-rs.PeerUpdates.Done():
		// NOTE: We explicitly DO NOT close the PeerUpdatesCh's updateCh go channel.
		// This is because there may be numerous spawned goroutines that are
		// attempting to send on the updateCh go channel and when the reactor stops
		// we do not want to preemptively close the channel as that could result in
		// panics sending on a closed channel. This also means that reactors MUST
		// be certain there are NO listeners on the updateCh channel when closing or
		// stopping.
	}
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

	msg := proto.Clone(channelShim.Channel.messageType)
	msg.Reset()

	if err := proto.Unmarshal(msgBytes, msg); err != nil {
		rs.Logger.Error("error decoding message", "peer", src, "ch_id", cID, "err", err)
		rs.Switch.StopPeerForError(src, err)
		return
	}

	validator, ok := msg.(messageValidator)
	if ok {
		if err := validator.Validate(); err != nil {
			rs.Logger.Error("invalid message", "peer", src, "ch_id", cID, "err", err)
			rs.Switch.StopPeerForError(src, err)
			return
		}
	}

	wrapper, ok := msg.(Wrapper)
	if ok {
		var err error

		msg, err = wrapper.Unwrap()
		if err != nil {
			rs.Logger.Error("failed to unwrap message", "peer", src, "ch_id", chID, "err", err)
			return
		}
	}

	select {
	case channelShim.inCh <- Envelope{From: src.ID(), Message: msg}:
		rs.Logger.Debug("proxied envelope", "reactor", rs.Name, "ch_id", cID, "peer", src.ID())

	case <-channelShim.Channel.Done():
		// NOTE: We explicitly DO NOT close the p2p Channel's inbound go channel.
		// This is because there may be numerous spawned goroutines that are
		// attempting to send on the inbound channel and when the reactor stops we
		// do not want to preemptively close the channel as that could result in
		// panics sending on a closed channel. This also means that reactors MUST
		// be certain there are NO listeners on the inbound channel when closing or
		// stopping.
	}
}
