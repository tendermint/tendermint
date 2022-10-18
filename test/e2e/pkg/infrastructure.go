package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// InfrastructureData contains the relevant information for a set of existing
// infrastructure that is to be used for running a testnet.
type InfrastructureData struct {

	// Provider is the name of infrastructure provider backing the testnet.
	// For example, 'docker' if it is running locally in a docker network or
	// 'digital-ocean', 'aws', 'google', etc. if it is from a cloud provider.
	Provider string `json:"provider"`

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
	netAddress := dockerIPv4CIDR
	if m.IPv6 {
		netAddress = dockerIPv6CIDR
	}
	_, ipNet, err := net.ParseCIDR(netAddress)
	if err != nil {
		return InfrastructureData{}, fmt.Errorf("invalid IP network address %q: %w", netAddress, err)
	}
	ipGen := newIPGenerator(ipNet)
	ifd := InfrastructureData{
		Provider:  "docker",
		Instances: make(map[string]InstanceData),
	}
	for name := range m.Nodes {
		ifd.Instances[name] = InstanceData{
			IPAddress: ipGen.Next(),
		}
	}
	return ifd, nil
}

func InfrastructureDataFromFile(p string) (InfrastructureData, error) {
	ifd := InfrastructureData{}
	b, err := os.ReadFile(p)
	err = json.Unmarshal(b, &ifd)
	if err != nil {
		return InfrastructureData{}, err
	}
	return ifd, nil
}
