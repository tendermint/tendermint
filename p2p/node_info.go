package p2p

import (
	"fmt"
	"strings"

	crypto "github.com/tendermint/go-crypto"
)

const (
	maxNodeInfoSize = 10240 // 10Kb
	maxNumChannels  = 16    // plenty of room for upgrades, for now
)

func MaxNodeInfoSize() int {
	return maxNodeInfoSize
}

// NodeInfo is the basic node information exchanged
// between two peers during the Tendermint P2P handshake.
type NodeInfo struct {
	// Authenticate
	PubKey     crypto.PubKey `json:"pub_key"`     // authenticated pubkey
	ListenAddr string        `json:"listen_addr"` // accepting incoming

	// Check compatibility
	Network  string `json:"network"`  // network/chain ID
	Version  string `json:"version"`  // major.minor.revision
	Channels []byte `json:"channels"` // channels this node knows about

	// Sanitize
	Moniker string   `json:"moniker"` // arbitrary moniker
	Other   []string `json:"other"`   // other application specific data
}

// Validate checks the self-reported NodeInfo is safe.
// It returns an error if there
// are too many Channels or any duplicate Channels.
// TODO: constraints for Moniker/Other? Or is that for the UI ?
func (info NodeInfo) Validate() error {
	if len(info.Channels) > maxNumChannels {
		return fmt.Errorf("info.Channels is too long (%v). Max is %v", len(info.Channels), maxNumChannels)
	}

	channels := make(map[byte]struct{})
	for _, ch := range info.Channels {
		_, ok := channels[ch]
		if ok {
			return fmt.Errorf("info.Channels contains duplicate channel id %v", ch)
		}
		channels[ch] = struct{}{}
	}
	return nil
}

// CompatibleWith checks if two NodeInfo are compatible with eachother.
// CONTRACT: two nodes are compatible if the major/minor versions match and network match
// and they have at least one channel in common.
func (info NodeInfo) CompatibleWith(other NodeInfo) error {
	iMajor, iMinor, _, iErr := splitVersion(info.Version)
	oMajor, oMinor, _, oErr := splitVersion(other.Version)

	// if our own version number is not formatted right, we messed up
	if iErr != nil {
		return iErr
	}

	// version number must be formatted correctly ("x.x.x")
	if oErr != nil {
		return oErr
	}

	// major version must match
	if iMajor != oMajor {
		return fmt.Errorf("Peer is on a different major version. Got %v, expected %v", oMajor, iMajor)
	}

	// minor version must match
	if iMinor != oMinor {
		return fmt.Errorf("Peer is on a different minor version. Got %v, expected %v", oMinor, iMinor)
	}

	// nodes must be on the same network
	if info.Network != other.Network {
		return fmt.Errorf("Peer is on a different network. Got %v, expected %v", other.Network, info.Network)
	}

	// if we have no channels, we're just testing
	if len(info.Channels) == 0 {
		return nil
	}

	// for each of our channels, check if they have it
	found := false
OUTER_LOOP:
	for _, ch1 := range info.Channels {
		for _, ch2 := range other.Channels {
			if ch1 == ch2 {
				found = true
				break OUTER_LOOP // only need one
			}
		}
	}
	if !found {
		return fmt.Errorf("Peer has no common channels. Our channels: %v ; Peer channels: %v", info.Channels, other.Channels)
	}
	return nil
}

// ID returns node's ID.
func (info NodeInfo) ID() ID {
	return PubKeyToID(info.PubKey)
}

// NetAddress returns a NetAddress derived from the NodeInfo -
// it includes the authenticated peer ID and the self-reported
// ListenAddr. Note that the ListenAddr is not authenticated and
// may not match that address actually dialed if its an outbound peer.
func (info NodeInfo) NetAddress() *NetAddress {
	id := PubKeyToID(info.PubKey)
	addr := info.ListenAddr
	netAddr, err := NewNetAddressString(IDAddressString(id, addr))
	if err != nil {
		panic(err) // everything should be well formed by now
	}
	return netAddr
}

func (info NodeInfo) String() string {
	return fmt.Sprintf("NodeInfo{pk: %v, moniker: %v, network: %v [listen %v], version: %v (%v)}", info.PubKey, info.Moniker, info.Network, info.ListenAddr, info.Version, info.Other)
}

func splitVersion(version string) (string, string, string, error) {
	spl := strings.Split(version, ".")
	if len(spl) != 3 {
		return "", "", "", fmt.Errorf("Invalid version format %v", version)
	}
	return spl[0], spl[1], spl[2], nil
}
