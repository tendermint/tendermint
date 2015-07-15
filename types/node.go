package types

import (
	"fmt"
	acm "github.com/tendermint/tendermint/account"
	"strings"
)

type NodeInfo struct {
	PubKey   acm.PubKeyEd25519 `json:"pub_key"`
	Moniker  string            `json:"moniker"`
	ChainID  string            `json:"chain_id"`
	Version  string            `json:"version"`
	Revision string            `json:"revision"`
	Host     string            `json:"host"`
	P2PPort  uint16            `json:"p2p_port"`
	RPCPort  uint16            `json:"rpc_port"`
}

func (ni *NodeInfo) CompatibleWith(no *NodeInfo) error {
	iM, im, _, ie := splitVersion(ni.Version)
	oM, om, _, oe := splitVersion(no.Version)

	// if our own version number is not formatted right, we messed up
	if ie != nil {
		return ie
	}

	// version number must be formatted correctly ("x.x.x")
	if oe != nil {
		return oe
	}

	// major version must match
	if iM != oM {
		return fmt.Errorf("Peer is on a different major version. Got %v, expected %v", oM, iM)
	}

	// minor version must match
	if im != om {
		return fmt.Errorf("Peer is on a different minor version. Got %v, expected %v", om, im)
	}

	// nodes must be on the same chain_id
	if ni.ChainID != no.ChainID {
		return fmt.Errorf("Peer is on a different chain_id. Got %v, expected %v", no.ChainID, ni.ChainID)
	}

	return nil
}

func splitVersion(version string) (string, string, string, error) {
	spl := strings.Split(version, ".")
	if len(spl) != 3 {
		return "", "", "", fmt.Errorf("Invalid version format %v", version)
	}
	return spl[0], spl[1], spl[2], nil
}
