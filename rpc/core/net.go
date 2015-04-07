package core

import (
	"github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tendermint/db"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

func Status() (*ctypes.ResponseStatus, error) {
	db := dbm.NewMemDB()
	genesisState := sm.MakeGenesisStateFromFile(db, config.App().GetString("GenesisFile"))
	genesisHash := genesisState.Hash()
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

	return &ctypes.ResponseStatus{genesisHash, config.App().GetString("Network"), latestBlockHash, latestHeight, latestBlockTime}, nil
}

//-----------------------------------------------------------------------------

func NetInfo() (*ctypes.ResponseNetInfo, error) {
	listening := p2pSwitch.IsListening()
	network := config.App().GetString("Network")
	listeners := []string{}
	for _, listener := range p2pSwitch.Listeners() {
		listeners = append(listeners, listener.String())
	}
	peers := []string{}
	for _, peer := range p2pSwitch.Peers().List() {
		peers = append(peers, peer.String())
	}
	return &ctypes.ResponseNetInfo{
		Network:   network,
		Listening: listening,
		Listeners: listeners,
		Peers:     peers,
	}, nil
}
