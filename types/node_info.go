package types

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/libs/bytes"
	tmstrings "github.com/tendermint/tendermint/libs/strings"
	tmp2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
)

const (
	maxNodeInfoSize = 10240 // 10KB
	maxNumChannels  = 16    // plenty of room for upgrades, for now
)

// Max size of the NodeInfo struct
func MaxNodeInfoSize() int {
	return maxNodeInfoSize
}

// ProtocolVersion contains the protocol versions for the software.
type ProtocolVersion struct {
	P2P   uint64 `json:"p2p,string"`
	Block uint64 `json:"block,string"`
	App   uint64 `json:"app,string"`
}

//-------------------------------------------------------------

// NodeInfo is the basic node information exchanged
// between two peers during the Tendermint P2P handshake.
type NodeInfo struct {
	ProtocolVersion ProtocolVersion `json:"protocol_version"`

	// Authenticate
	NodeID     NodeID `json:"id"`          // authenticated identifier
	ListenAddr string `json:"listen_addr"` // accepting incoming

	// Check compatibility.
	// Channels are HexBytes so easier to read as JSON
	Network string `json:"network"` // network/chain ID
	Version string `json:"version"` // major.minor.revision
	// FIXME: This should be changed to uint16 to be consistent with the updated channel type
	Channels bytes.HexBytes `json:"channels"` // channels this node knows about

	// ASCIIText fields
	Moniker string        `json:"moniker"` // arbitrary moniker
	Other   NodeInfoOther `json:"other"`   // other application specific data
}

// NodeInfoOther is the misc. applcation specific data
type NodeInfoOther struct {
	TxIndex    string `json:"tx_index"`
	RPCAddress string `json:"rpc_address"`
}

// ID returns the node's peer ID.
func (info NodeInfo) ID() NodeID {
	return info.NodeID
}

// Validate checks the self-reported NodeInfo is safe.
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
func (info NodeInfo) Validate() error {
	if _, _, err := ParseAddressString(info.ID().AddressString(info.ListenAddr)); err != nil {
		return err
	}

	// Validate Version
	if len(info.Version) > 0 {
		if ver, err := tmstrings.ASCIITrim(info.Version); err != nil || ver == "" {
			return fmt.Errorf("info.Version must be valid ASCII text without tabs, but got, %q [%s]", info.Version, ver)
		}
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

	if m, err := tmstrings.ASCIITrim(info.Moniker); err != nil || m == "" {
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
	if len(rpcAddr) > 0 {
		if a, err := tmstrings.ASCIITrim(rpcAddr); err != nil || a == "" {
			return fmt.Errorf("info.Other.RPCAddress=%v must be valid ASCII text without tabs", rpcAddr)
		}
	}

	return nil
}

// CompatibleWith checks if two NodeInfo are compatible with each other.
// CONTRACT: two nodes are compatible if the Block version and network match
// and they have at least one channel in common.
func (info NodeInfo) CompatibleWith(other NodeInfo) error {
	if info.ProtocolVersion.Block != other.ProtocolVersion.Block {
		return fmt.Errorf("peer is on a different Block version. Got %v, expected %v",
			other.ProtocolVersion.Block, info.ProtocolVersion.Block)
	}

	// nodes must be on the same network
	if info.Network != other.Network {
		return fmt.Errorf("peer is on a different network. Got %v, expected %v", other.Network, info.Network)
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
		return fmt.Errorf("peer has no common channels. Our channels: %v ; Peer channels: %v", info.Channels, other.Channels)
	}
	return nil
}

// AddChannel is used by the router when a channel is opened to add it to the node info
func (info *NodeInfo) AddChannel(channel uint16) {
	// check that the channel doesn't already exist
	for _, ch := range info.Channels {
		if ch == byte(channel) {
			return
		}
	}

	info.Channels = append(info.Channels, byte(channel))
}

func (info NodeInfo) Copy() NodeInfo {
	return NodeInfo{
		ProtocolVersion: info.ProtocolVersion,
		NodeID:          info.NodeID,
		ListenAddr:      info.ListenAddr,
		Network:         info.Network,
		Version:         info.Version,
		Channels:        info.Channels,
		Moniker:         info.Moniker,
		Other:           info.Other,
	}
}

func (info NodeInfo) ToProto() *tmp2p.NodeInfo {

	dni := new(tmp2p.NodeInfo)
	dni.ProtocolVersion = tmp2p.ProtocolVersion{
		P2P:   info.ProtocolVersion.P2P,
		Block: info.ProtocolVersion.Block,
		App:   info.ProtocolVersion.App,
	}

	dni.NodeID = string(info.NodeID)
	dni.ListenAddr = info.ListenAddr
	dni.Network = info.Network
	dni.Version = info.Version
	dni.Channels = info.Channels
	dni.Moniker = info.Moniker
	dni.Other = tmp2p.NodeInfoOther{
		TxIndex:    info.Other.TxIndex,
		RPCAddress: info.Other.RPCAddress,
	}

	return dni
}

func NodeInfoFromProto(pb *tmp2p.NodeInfo) (NodeInfo, error) {
	if pb == nil {
		return NodeInfo{}, errors.New("nil node info")
	}
	dni := NodeInfo{
		ProtocolVersion: ProtocolVersion{
			P2P:   pb.ProtocolVersion.P2P,
			Block: pb.ProtocolVersion.Block,
			App:   pb.ProtocolVersion.App,
		},
		NodeID:     NodeID(pb.NodeID),
		ListenAddr: pb.ListenAddr,
		Network:    pb.Network,
		Version:    pb.Version,
		Channels:   pb.Channels,
		Moniker:    pb.Moniker,
		Other: NodeInfoOther{
			TxIndex:    pb.Other.TxIndex,
			RPCAddress: pb.Other.RPCAddress,
		},
	}

	return dni, nil
}

// ParseAddressString reads an address string, and returns the IP
// address and port information, returning an error for any validation
// errors.
func ParseAddressString(addr string) (net.IP, uint16, error) {
	addrWithoutProtocol := removeProtocolIfDefined(addr)
	spl := strings.Split(addrWithoutProtocol, "@")
	if len(spl) != 2 {
		return nil, 0, errors.New("invalid address")
	}

	id, err := NewNodeID(spl[0])
	if err != nil {
		return nil, 0, err
	}

	if err := id.Validate(); err != nil {
		return nil, 0, err
	}

	addrWithoutProtocol = spl[1]

	// get host and port
	host, portStr, err := net.SplitHostPort(addrWithoutProtocol)
	if err != nil {
		return nil, 0, err
	}
	if len(host) == 0 {
		return nil, 0, err
	}

	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, 0, err
		}
		ip = ips[0]
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, 0, err
	}

	return ip, uint16(port), nil
}

func removeProtocolIfDefined(addr string) string {
	if strings.Contains(addr, "://") {
		return strings.Split(addr, "://")[1]
	}
	return addr

}
