package p2p

import (
	"fmt"
	"reflect"
	"strings"

	cmn "github.com/tendermint/tendermint/libs/common"
)

const (
	maxNodeInfoSize = 10240 // 10Kb
	maxNumChannels  = 16    // plenty of room for upgrades, for now
)

// Max size of the NodeInfo struct
func MaxNodeInfoSize() int {
	return maxNodeInfoSize
}

type NodeInfo interface {
	ID() ID
	ValidateBasic() error
	CompatibleWith(other NodeInfo) error
	NetAddress() *NetAddress
	String() string
}

// DefaultNodeInfo is the basic node information exchanged
// between two peers during the Tendermint P2P handshake.
type DefaultNodeInfo struct {
	// Authenticate
	// TODO: replace with NetAddress
	ID_        ID     `json:"id"`          // authenticated identifier
	ListenAddr string `json:"listen_addr"` // accepting incoming

	// Check compatibility.
	// Channels are HexBytes so easier to read as JSON
	Network  string       `json:"network"`  // network/chain ID
	Version  string       `json:"version"`  // major.minor.revision
	Channels cmn.HexBytes `json:"channels"` // channels this node knows about

	// ASCIIText fields
	Moniker string               `json:"moniker"` // arbitrary moniker
	Other   DefaultNodeInfoOther `json:"other"`   // other application specific data
}

// DefaultNodeInfoOther is the misc. applcation specific data
type DefaultNodeInfoOther struct {
	AminoVersion     string `json:"amino_version"`
	P2PVersion       string `json:"p2p_version"`
	ConsensusVersion string `json:"consensus_version"`
	RPCVersion       string `json:"rpc_version"`
	TxIndex          string `json:"tx_index"`
	RPCAddress       string `json:"rpc_address"`
}

func (o DefaultNodeInfoOther) String() string {
	return fmt.Sprintf(
		"{amino_version: %v, p2p_version: %v, consensus_version: %v, rpc_version: %v, tx_index: %v, rpc_address: %v}",
		o.AminoVersion,
		o.P2PVersion,
		o.ConsensusVersion,
		o.RPCVersion,
		o.TxIndex,
		o.RPCAddress,
	)
}

// ID returns the node's peer ID.
func (info DefaultNodeInfo) ID() ID {
	return info.ID_
}

// ValidateBasic checks the self-reported DefaultNodeInfo is safe.
// It returns an error if there
// are too many Channels, if there are any duplicate Channels,
// if the ListenAddr is malformed, or if the ListenAddr is a host name
// that can not be resolved to some IP.
// TODO: constraints for Moniker/Other? Or is that for the UI ?
// JAE: It needs to be done on the client, but to prevent ambiguous
// unicode characters, maybe it's worth sanitizing it here.
// In the future we might want to validate these, once we have a
// name-resolution system up.
// International clients could then use punycode (or we could use
// url-encoding), and we just need to be careful with how we handle that in our
// clients. (e.g. off by default).
func (info DefaultNodeInfo) ValidateBasic() error {
	if len(info.Channels) > maxNumChannels {
		return fmt.Errorf("info.Channels is too long (%v). Max is %v", len(info.Channels), maxNumChannels)
	}

	// Sanitize ASCII text fields.
	if !cmn.IsASCIIText(info.Moniker) || cmn.ASCIITrim(info.Moniker) == "" {
		return fmt.Errorf("info.Moniker must be valid non-empty ASCII text without tabs, but got %v", info.Moniker)
	}

	// Sanitize versions
	// XXX: Should we be more strict about version and address formats?
	other := info.Other
	versions := []string{
		other.AminoVersion,
		other.P2PVersion,
		other.ConsensusVersion,
		other.RPCVersion}
	for i, v := range versions {
		if cmn.ASCIITrim(v) != "" && !cmn.IsASCIIText(v) {
			return fmt.Errorf("info.Other[%d]=%v must be valid non-empty ASCII text without tabs", i, v)
		}
	}
	if cmn.ASCIITrim(other.TxIndex) != "" && (other.TxIndex != "on" && other.TxIndex != "off") {
		return fmt.Errorf("info.Other.TxIndex should be either 'on' or 'off', got '%v'", other.TxIndex)
	}
	if cmn.ASCIITrim(other.RPCAddress) != "" && !cmn.IsASCIIText(other.RPCAddress) {
		return fmt.Errorf("info.Other.RPCAddress=%v must be valid non-empty ASCII text without tabs", other.RPCAddress)
	}

	channels := make(map[byte]struct{})
	for _, ch := range info.Channels {
		_, ok := channels[ch]
		if ok {
			return fmt.Errorf("info.Channels contains duplicate channel id %v", ch)
		}
		channels[ch] = struct{}{}
	}

	// ensure ListenAddr is good
	_, err := NewNetAddressString(IDAddressString(info.ID(), info.ListenAddr))
	return err
}

// CompatibleWith checks if two DefaultNodeInfo are compatible with eachother.
// CONTRACT: two nodes are compatible if the major version matches and network match
// and they have at least one channel in common.
func (info DefaultNodeInfo) CompatibleWith(other_ NodeInfo) error {
	other, ok := other_.(DefaultNodeInfo)
	if !ok {
		return fmt.Errorf("wrong NodeInfo type. Expected DefaultNodeInfo, got %v", reflect.TypeOf(other_))
	}

	iMajor, _, _, iErr := splitVersion(info.Version)
	oMajor, _, _, oErr := splitVersion(other.Version)

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

// NetAddress returns a NetAddress derived from the DefaultNodeInfo -
// it includes the authenticated peer ID and the self-reported
// ListenAddr. Note that the ListenAddr is not authenticated and
// may not match that address actually dialed if its an outbound peer.
func (info DefaultNodeInfo) NetAddress() *NetAddress {
	netAddr, err := NewNetAddressString(IDAddressString(info.ID(), info.ListenAddr))
	if err != nil {
		switch err.(type) {
		case ErrNetAddressLookup:
			// XXX If the peer provided a host name  and the lookup fails here
			// we're out of luck.
			// TODO: use a NetAddress in DefaultNodeInfo
		default:
			panic(err) // everything should be well formed by now
		}
	}
	return netAddr
}

func (info DefaultNodeInfo) String() string {
	return fmt.Sprintf("DefaultNodeInfo{id: %v, moniker: %v, network: %v [listen %v], version: %v (%v)}",
		info.ID_, info.Moniker, info.Network, info.ListenAddr, info.Version, info.Other)
}

func splitVersion(version string) (string, string, string, error) {
	spl := strings.Split(version, ".")
	if len(spl) != 3 {
		return "", "", "", fmt.Errorf("Invalid version format %v", version)
	}
	return spl[0], spl[1], spl[2], nil
}
