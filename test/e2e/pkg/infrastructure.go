package e2e

import "net"

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
