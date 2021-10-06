package p2p

import (
	"sort"
)

// ReactorShim defines a generic shim wrapper around a BaseReactor. It is
// responsible for wiring up legacy p2p behavior to the new p2p semantics
// (e.g. proxying Envelope messages to legacy peers).
type ReactorShim struct {
	Name     string
	Channels map[ChannelID]*Channel
}

func NewReactorShim(name string, descriptors map[ChannelID]ChannelDescriptor) *ReactorShim {
	channels := make(map[ChannelID]*Channel)

	for _, cds := range descriptors {
		ch := makeChannel(cds, 0)
		channels[ch.ID] = ch
	}

	rs := &ReactorShim{
		Name:     name,
		Channels: channels,
	}

	return rs
}

func makeChannel(cd ChannelDescriptor, buf uint) *Channel {
	return NewChannel(
		cd,
		cd.MsgType,
		make(chan Envelope, buf),  // in-channel
		make(chan Envelope, buf),  // out-channel
		make(chan PeerError, buf), // err-channel
	)
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
		descriptors[i] = &rs.Channels[cID].descriptor
	}

	return descriptors
}
