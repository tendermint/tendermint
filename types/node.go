package types

import (
	"fmt"
	"strings"
)

type NodeInfo struct {
	Moniker string
	Network string
	Version string

	Host    string
	P2PPort uint16
	RPCPort uint16
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

	// nodes must be on the same network
	if ni.Network != no.Network {
		return fmt.Errorf("Peer is on a different network. Got %v, expected %v", no.Network, ni.Network)
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
