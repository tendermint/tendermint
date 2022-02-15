package quorum

import (
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

// nodeAddress converts ValidatorAddress to a NodeAddress object
func nodeAddress(va types.ValidatorAddress) p2p.NodeAddress {
	nodeAddress := p2p.NodeAddress{
		NodeID:   va.NodeID,
		Protocol: p2p.TCPProtocol,
		Hostname: va.Hostname,
		Port:     va.Port,
	}
	return nodeAddress
}
