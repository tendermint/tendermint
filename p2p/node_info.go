package p2p

import (
	"fmt"
	"reflect"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/version"
)

const (
	maxNodeInfoSize = 10240 // 10Kb
	maxNumChannels  = 16    // plenty of room for upgrades, for now
)

// Max size of the NodeInfo struct
func MaxNodeInfoSize() int {
	return maxNodeInfoSize
}

//-------------------------------------------------------------

// NodeInfo exposes basic info of a node
// and determines if we're compatible.
type NodeInfo interface {
	nodeInfoAddress
	nodeInfoTransport
}

// nodeInfoAddress exposes just the core info of a node.
type nodeInfoAddress interface {
	ID() ID
	NetAddress() *NetAddress
}

// nodeInfoTransport validates a nodeInfo and checks
// our compatibility with it. It's for use in the handshake.
type nodeInfoTransport interface {
	Validate() error
	CompatibleWith(other NodeInfo) error
}

//-------------------------------------------------------------

// ProtocolVersion contains the protocol versions for the software.
type ProtocolVersion struct {
	P2P   version.Protocol `json:"p2p"`
	Block version.Protocol `json:"block"`
	App   version.Protocol `json:"app"`
}

// defaultProtocolVersion populates the Block and P2P versions using
// the global values, but not the App.
var defaultProtocolVersion = NewProtocolVersion(
	version.P2PProtocol,
	version.BlockProtocol,
	0,
)

// NewProtocolVersion returns a fully populated ProtocolVersion.
func NewProtocolVersion(p2p, block, app version.Protocol) ProtocolVersion {
	return ProtocolVersion{
		P2P:   p2p,
		Block: block,
		App:   app,
	}
}

//-------------------------------------------------------------

// Assert DefaultNodeInfo satisfies NodeInfo
var _ NodeInfo = DefaultNodeInfo{}

// DefaultNodeInfo is the basic node information exchanged
// between two peers during the Tendermint P2P handshake.
type DefaultNodeInfo struct {
	ProtocolVersion ProtocolVersion `json:"protocol_version"`

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
	TxIndex    string `json:"tx_index"`
	RPCAddress string `json:"rpc_address"`
}

// ID returns the node's peer ID.
func (info DefaultNodeInfo) ID() ID {
	return info.ID_
}

// Validate checks the self-reported DefaultNodeInfo is safe.
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
func (info DefaultNodeInfo) Validate() error {

	// ID is already validated.

	// Validate ListenAddr.
	_, err := NewNetAddressString(IDAddressString(info.ID(), info.ListenAddr))
	if err != nil {
		return err
	}

	// Network is validated in CompatibleWith.

	// Validate Version
	if len(info.Version) > 0 &&
		(!cmn.IsASCIIText(info.Version) || cmn.ASCIITrim(info.Version) == "") {

		return fmt.Errorf("info.Version must be valid ASCII text without tabs, but got %v", info.Version)
	}

	// Validate Channels - ensure max and check for duplicates.
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

	// Validate Moniker.
	if !cmn.IsASCIIText(info.Moniker) || cmn.ASCIITrim(info.Moniker) == "" {
		return fmt.Errorf("info.Moniker must be valid non-empty ASCII text without tabs, but got %v", info.Moniker)
	}

	// Validate Other.
	other := info.Other
	txIndex := other.TxIndex
	switch txIndex {
	case "", "on", "off":
	default:
		return fmt.Errorf("info.Other.TxIndex should be either 'on', 'off', or empty string, got '%v'", txIndex)
	}
	// XXX: Should we be more strict about address formats?
	rpcAddr := other.RPCAddress
	if len(rpcAddr) > 0 && (!cmn.IsASCIIText(rpcAddr) || cmn.ASCIITrim(rpcAddr) == "") {
		return fmt.Errorf("info.Other.RPCAddress=%v must be valid ASCII text without tabs", rpcAddr)
	}

	return nil
}

// CompatibleWith checks if two DefaultNodeInfo are compatible with eachother.
// CONTRACT: two nodes are compatible if the Block version and network match
// and they have at least one channel in common.
func (info DefaultNodeInfo) CompatibleWith(other_ NodeInfo) error {
	other, ok := other_.(DefaultNodeInfo)
	if !ok {
		return fmt.Errorf("wrong NodeInfo type. Expected DefaultNodeInfo, got %v", reflect.TypeOf(other_))
	}

	if info.ProtocolVersion.Block != other.ProtocolVersion.Block {
		return fmt.Errorf("Peer is on a different Block version. Got %v, expected %v",
			other.ProtocolVersion.Block, info.ProtocolVersion.Block)
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
	idAddr := IDAddressString(info.ID(), info.ListenAddr)
	netAddr, err := NewNetAddressString(idAddr)
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

//-----------------------------------------------------------
// These methods are for Protobuf Compatibility

// Size returns the size of the amino encoding, in bytes.
func (info *DefaultNodeInfo) Size() int {
	bs, _ := info.Marshal()
	return len(bs)
}

// Marshal returns the amino encoding.
func (info *DefaultNodeInfo) Marshal() ([]byte, error) {
	return cdc.MarshalBinaryBare(info)
}

// MarshalTo calls Marshal and copies to the given buffer.
func (info *DefaultNodeInfo) MarshalTo(data []byte) (int, error) {
	bs, err := info.Marshal()
	if err != nil {
		return -1, err
	}
	return copy(data, bs), nil
}

// Unmarshal deserializes from amino encoded form.
func (info *DefaultNodeInfo) Unmarshal(bs []byte) error {
	return cdc.UnmarshalBinaryBare(bs, info)
}
