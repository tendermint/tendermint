package core

import (
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

// TODO Move to status.go or node.go
func Status() (*ctypes.ResultStatus, error) {
	latestHeight := blockStore.Height()
	var (
		latestBlockMeta *types.BlockMeta
		latestBlockHash []byte
		latestBlockTime int64
	)
	if latestHeight != 0 {
		latestBlockMeta = blockStore.LoadBlockMeta(latestHeight)
		latestBlockHash = latestBlockMeta.Hash
		latestBlockTime = latestBlockMeta.Header.Time.UnixNano()
	}

	return &ctypes.ResultStatus{
		NodeInfo:          p2pSwitch.NodeInfo(),
		PubKey:            privValidator.PubKey,
		LatestBlockHash:   latestBlockHash,
		LatestBlockHeight: latestHeight,
		LatestBlockTime:   latestBlockTime}, nil
}

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
			NodeInfo:   *peer.NodeInfo,
			IsOutbound: peer.IsOutbound(),
		})
	}
	return &ctypes.ResultNetInfo{
		Listening: listening,
		Listeners: listeners,
		Peers:     peers,
	}, nil
}

//-----------------------------------------------------------------------------

func Genesis() (*ctypes.ResultGenesis, error) {
	return &ctypes.ResultGenesis{genDoc}, nil
}
