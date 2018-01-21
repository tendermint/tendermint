package p2p

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	crypto "github.com/tendermint/go-crypto"
)

const maxNodeInfoSize = 10240 // 10Kb

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
	Network string `json:"network"` // network/chain ID
	Version string `json:"version"` // major.minor.revision

	// Sanitize
	Moniker string   `json:"moniker"` // arbitrary moniker
	Other   []string `json:"other"`   // other application specific data
}

// Validate checks the self-reported NodeInfo is safe.
// It returns an error if the info.PubKey doesn't match the given pubKey.
// TODO: constraints for Moniker/Other? Or is that for the UI ?
func (info NodeInfo) Validate(pubKey crypto.PubKey) error {
	if !info.PubKey.Equals(pubKey) {
		return fmt.Errorf("info.PubKey (%v) doesn't match peer.PubKey (%v)",
			info.PubKey, pubKey)
	}
	return nil
}

// CONTRACT: two nodes are compatible if the major/minor versions match and network match
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

	return nil
}

func (info NodeInfo) ID() ID {
	return PubKeyToID(info.PubKey)
}

func (info NodeInfo) NetAddress() *NetAddress {
	id := PubKeyToID(info.PubKey)
	addr := info.ListenAddr
	netAddr, err := NewNetAddressString(IDAddressString(id, addr))
	if err != nil {
		panic(err) // everything should be well formed by now
	}
	return netAddr
}

func (info NodeInfo) ListenHost() string {
	host, _, _ := net.SplitHostPort(info.ListenAddr) // nolint: errcheck, gas
	return host
}

func (info NodeInfo) ListenPort() int {
	_, port, _ := net.SplitHostPort(info.ListenAddr) // nolint: errcheck, gas
	port_i, err := strconv.Atoi(port)
	if err != nil {
		return -1
	}
	return port_i
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
