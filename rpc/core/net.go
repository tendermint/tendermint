package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

//-----------------------------------------------------------------------------

func NetInfo() (*ctypes.ResultNetInfo, error) {
	listening := p2pSwitch.IsListening()
	listeners := []string{}
	for _, listener := range p2pSwitch.Listeners() {
		listeners = append(listeners, listener.String())
	}
	peers := []ctypes.Peer{}
	for _, peer := range p2pSwitch.Peers().List() {
		peers = append(peers, ctypes.Peer{
			NodeInfo:         *peer.NodeInfo,
			IsOutbound:       peer.IsOutbound(),
			ConnectionStatus: peer.Connection().Status(),
		})
	}
	return &ctypes.ResultNetInfo{
		Listening: listening,
		Listeners: listeners,
		Peers:     peers,
	}, nil
}

//-----------------------------------------------------------------------------

// Dial given list of seeds
func UnsafeDialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {

	if len(seeds) == 0 {
		return &ctypes.ResultDialSeeds{}, fmt.Errorf("No seeds provided")
	}
	// starts go routines to dial each seed after random delays
	logger.Info("DialSeeds", "addrBook", addrBook, "seeds", seeds)
	err := p2pSwitch.DialSeeds(addrBook, seeds)
	if err != nil {
		return &ctypes.ResultDialSeeds{}, err
	}
	return &ctypes.ResultDialSeeds{"Dialing seeds in progress. See /net_info for details"}, nil
}

//-----------------------------------------------------------------------------

func Genesis() (*ctypes.ResultGenesis, error) {
	return &ctypes.ResultGenesis{genDoc}, nil
}
