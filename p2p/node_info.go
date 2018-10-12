package p2p

import (
	"fmt"
	"reflect"
	"strings"

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

// NodeInfo exposes basic info of a node
// and determines if we're compatible
type NodeInfo interface {
	nodeInfoAddress
	nodeInfoTransport
}

// nodeInfoAddress exposes just the core info of a node.
type nodeInfoAddress interface {
	ID() ID
	NetAddress() *NetAddress
}

// nodeInfoTransport is validates a nodeInfo and checks
// our compatibility with it. It's for use in the handshake.
type nodeInfoTransport interface {
	ValidateBasic() error
	CompatibleWith(other NodeInfo) error
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
func (info DefaultNodeInfo) CompatibleWith(otherInfo NodeInfo) error {
	other, ok := otherInfo.(DefaultNodeInfo)
	if !ok {
		return fmt.Errorf("wrong NodeInfo type. Expected DefaultNodeInfo, got %v", reflect.TypeOf(otherInfo))
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

func splitVersion(version string) (string, string, string, error) {
	spl := strings.Split(version, ".")
	if len(spl) != 3 {
		return "", "", "", fmt.Errorf("Invalid version format %v", version)
	}
	return spl[0], spl[1], spl[2], nil
}

//--------------------------------------------------------------------------------------

// NodeInfoV5 is the basic node information exchanged
// between two peers during the Tendermint P2P handshake.
type NodeInfoV5 struct {
	Version  VersionInfo `json:"version"`
	Address  AddressInfo `json:"id"`
	Network  NetworkInfo `json:"network"`
	Services ServiceInfo `json:"services"`
}

// VersionInfo contains all protocol and software version information for the node.
type VersionInfo struct {
	Protocol ProtocolVersion  `json:"protocol"`
	Software version.Software `json:"software"`
}

// ProtocolVersion contains the p2p, block, and app protocol versions.
type ProtocolVersion struct {
	P2P   version.Protocol `json:"p2p"`
	Block version.Protocol `json:"block"`

	// Don't bother with App for now until we can update it live
	// App   version.Protocol `json:"app"`
}

// AddressInfo contains info about the peers ID and network address.
type AddressInfo struct {
	// Authenticate
	// TODO: replace with NetAddress ?
	// Note NetAddress currently only has IP,
	// but we may want DNS name here.
	ID         ID     `json:"id"`          // authenticated identifier
	ListenAddr string `json:"listen_addr"` // accepting incoming

	// ASCIIText fields
	Moniker string `json:"moniker"` // arbitrary moniker
}

// NetworkInfo contains info about the network this peer is operating on.
// Currently, the only identifier is the ChainID, known here as the Name.
type NetworkInfo struct {
	Name string `json:"name"`
}

// ServiceInfo describes the services this peer offers to other peers and to users.
type ServiceInfo struct {
	Peers PeerServices `json:"peers`
	Users UserServices `json:"users"`
}

// PeerServices describes the services this peer offers to other peers,
// in terms of active Reactor channels.
type PeerServices struct {
	// Channels are HexBytes so easier to read as JSON
	Channels cmn.HexBytes `json:"channels"` // channels this node knows about
}

// UserServices describes the set of services exposed to the user.
type UserServices struct {
	TxIndex    string `json:"tx_index"`
	RPCAddress string `json:"rpc_address"`
}

//--------------------------------------------------------------------------

func (info NodeInfoV5) ID() ID {
	return info.Address.ID
}

func (info NodeInfoV5) ValidateBasic() error {

	if err := info.Version.Validate(); err != nil {
		return err
	}

	if err := info.Address.Validate(); err != nil {
		return err
	}

	if err := info.Network.Validate(); err != nil {
		return err
	}

	if err := info.Services.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks that the protocol versions are non-zero and that the software versions are ASCII.
func (info VersionInfo) Validate() error {
	//  TODO
	//	ProtocolVersion - {P2P, Block} greater than 0
	//	SoftwareVersion - ASCII
	return nil
}

// Validate checks that the ListenAddr is well formed and that the moniker is ASCII.
// The ID should have already been checked.
func (info AddressInfo) Validate() error {
	if _, err := sanitizeASCII(info.Moniker); err != nil {
		return fmt.Errorf("Moniker %v", err)
	}

	// ensure ListenAddr is good
	_, err := NewNetAddressString(IDAddressString(info.ID, info.ListenAddr))
	return err

}

// Validate checks that the NetworkInfo.Name is ASCII.
func (info NetworkInfo) Validate() error {
	if _, err := sanitizeASCII(info.Name); err != nil {
		return fmt.Errorf("Name %v", err)
	}
	return nil
}

// Validate validates the PeerServices and UserServices
func (info ServiceInfo) Validate() error {

	if err := info.Peers.Validate(); err != nil {
		return err
	}

	if err := info.Users.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks that there are not too many channels or any duplicate channels.
func (services PeerServices) Validate() error {
	channelBytes := services.Channels
	if len(channelBytes) > maxNumChannels {
		return fmt.Errorf("Channels is too long (%v). Max is %v", len(channelBytes), maxNumChannels)
	}

	channels := make(map[byte]struct{})
	for _, ch := range channelBytes {
		_, ok := channels[ch]
		if ok {
			return fmt.Errorf("Channels contains duplicate channel id %v", ch)
		}
		channels[ch] = struct{}{}
	}
	return nil
}

func (services UserServices) Validate() error {
	txIndex, err := sanitizeASCII(services.TxIndex)
	if err != nil {
		return fmt.Errorf("TxIndex %v", err)
	}

	if _, err := sanitizeASCII(services.RPCAddress); err != nil {
		return fmt.Errorf("RPCAddress %v", err)
	}

	switch cmn.ASCIITrim(txIndex) {
	case "on", "off":
		// do nothing
	default:
		return fmt.Errorf("TxIndex should be either 'on' or 'off', got '%v'", txIndex)
	}
	return nil
}

func sanitizeASCII(input string) (string, error) {
	if !cmn.IsASCIIText(input) || cmn.ASCIITrim(input) == "" {
		return "", fmt.Errorf("must be valid non-empty ASCII text without tabs, but got %v", input)
	}
	return cmn.ASCIITrim(input), nil
}

// CompatibleWith checks if two NodeInfoV5 are compatible with eachother.
// CONTRACT: two nodes are compatible if the major version matches and network match
// and they have at least one channel in common.
func (info NodeInfoV5) CompatibleWith(otherInfo NodeInfo) error {
	other, ok := otherInfo.(NodeInfoV5)
	if !ok {
		return fmt.Errorf("wrong NodeInfo type. Expected DefaultNodeInfo, got %v", reflect.TypeOf(otherInfo))
	}

	// if we have no channels, we're just testing
	ourChannels := info.Services.Peers.Channels
	otherChannels := other.Services.Peers.Channels
	if len(ourChannels) == 0 {
		return nil
	}

	// nodes must have the same block version
	if info.Version.Protocol.Block != other.Version.Protocol.Block {
		return fmt.Errorf(
			"Peer is running a different block protocol. Got %v, expected %v",
			other.Version.Protocol.Block,
			info.Version.Protocol.Block,
		)
	}

	// nodes must be on the same network
	if info.Network.Name != other.Network.Name {
		return fmt.Errorf("Peer is on a different network. Got %v, expected %v", other.Network.Name, info.Network.Name)
	}

	// for each of our channels, check if they have it
	found := false
OUTER_LOOP:
	for _, ch1 := range ourChannels {
		for _, ch2 := range otherChannels {
			if ch1 == ch2 {
				found = true
				break OUTER_LOOP // only need one
			}
		}
	}
	if !found {
		return fmt.Errorf("Peer has no common channels. Our channels: %v ; Peer channels: %v", ourChannels, otherChannels)
	}
	return nil
}

// NetAddress returns a NetAddress derived from the NodeInfoV5 -
// it includes the authenticated peer ID and the self-reported
// ListenAddr. Note that the ListenAddr is not authenticated and
// may not match that address actually dialed if its an outbound peer.
func (info NodeInfoV5) NetAddress() *NetAddress {
	netAddr, err := NewNetAddressString(IDAddressString(info.Address.ID, info.Address.ListenAddr))
	if err != nil {
		switch err.(type) {
		case ErrNetAddressLookup:
			// XXX If the peer provided a host name  and the lookup fails here
			// we're out of luck.
			// TODO: use a NetAddress in NodeInfoV5
		default:
			panic(err) // everything should be well formed by now
		}
	}
	return netAddr
}

func (info NodeInfoV5) String() string {
	return "TODO" // fmt.Sprintf("NodeInfoV5{id: %v, moniker: %v, network: %v [listen %v], version: %v (%v)}",
	// info.ID, info.Moniker, info.Network, info.ListenAddr, info.Version, info.Other)
}
