package core

import (
	"github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tendermint/db"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

func Status() (*ResponseStatus, error) {
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

	return &ResponseStatus{genesisHash, config.App().GetString("Network"), latestBlockHash, latestHeight, latestBlockTime}, nil
}

//-----------------------------------------------------------------------------

func NetInfo() (*ResponseNetInfo, error) {
	o, i, _ := p2pSwitch.NumPeers()
	numPeers := o + i
	listening := p2pSwitch.IsListening()
	network := config.App().GetString("Network")
	return &ResponseNetInfo{numPeers, listening, network}, nil
}
