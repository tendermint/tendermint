package e2e

import (
	"fmt"
	"net"
)

// InfrastructureData contains the relevant information for a set of existing
// infrastructure that is to be used for running a testnet.
type InfrastructureData struct {

	// Instances is a map of all of the machine instances on which to run
	// processes for a testnet.
	// The key of the map is the name of the instance, which each must correspond
	// to the names of one of the testnet nodes defined in the testnet manifest.
	Instances map[string]InstanceData `json:"instances"`
}

// InstanceData contains the relevant information for a machine instance backing
// one of the nodes in the testnet.
type InstanceData struct {
	IPAddress net.IP `json:"ip_address"`
}

func NewDockerInfrastructureData(m Manifest) (InfrastructureData, error) {
	netAddress := networkIPv4
	if m.IPv6 {
		netAddress = networkIPv6
	}
	_, ipNet, err := net.ParseCIDR(netAddress)
	if err != nil {
		return InfrastructureData{}, fmt.Errorf("invalid IP network address %q: %w", netAddress, err)
	}
	ipGen := newIPGenerator(ipNet)
	ifd := InfrastructureData{
		Instances: make(map[string]InstanceData),
	}
	for name := range m.Nodes {
		ifd.Instances[name] = InstanceData{
			IPAddress: ipGen.Next(),
		}
	}
	return ifd, nil
}
