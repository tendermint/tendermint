package p2p

import (
	"sort"

	"github.com/tendermint/tendermint/libs/log"
)

// ChannelDescriptorShim defines a shim wrapper around a legacy p2p channel
// and the proto.Message the new p2p Channel is responsible for handling.
// A ChannelDescriptorShim is not contained in ReactorShim, but is rather
// used to construct a ReactorShim.
type ChannelDescriptorShim struct {
	Descriptor *ChannelDescriptor
}

// ChannelShim defines a generic shim wrapper around a legacy p2p channel
// and the new p2p Channel. It also includes the raw bi-directional Go channels
// so we can proxy message delivery.
type ChannelShim struct {
	Descriptor *ChannelDescriptor
	Channel    *Channel
	inCh       chan<- Envelope
	outCh      <-chan Envelope
	errCh      <-chan PeerError
}

// ReactorShim defines a generic shim wrapper around a BaseReactor. It is
// responsible for wiring up legacy p2p behavior to the new p2p semantics
// (e.g. proxying Envelope messages to legacy peers).
type ReactorShim struct {
	Name        string
	PeerUpdates *PeerUpdates
	Channels    map[ChannelID]*ChannelShim
}

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
			cds.Descriptor.MessageType,
			inCh,
			outCh,
			errCh,
		),
		inCh:  inCh,
		outCh: outCh,
		errCh: errCh,
	}
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

// GetChannel returns a p2p Channel reference for a given ChannelID. If no
// Channel exists, nil is returned.
func (rs *ReactorShim) GetChannel(cID ChannelID) *Channel {
	channelShim, ok := rs.Channels[cID]
	if ok {
		return channelShim.Channel
	}

	return nil
}
