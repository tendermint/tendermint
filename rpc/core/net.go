package core

import (
	"github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tendermint/db"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

func Status() ([]byte, string, []byte, uint, int64) {
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

	return genesisHash, config.App().GetString("Network"), latestBlockHash, latestHeight, latestBlockTime
}

//-----------------------------------------------------------------------------

func NetInfo() (int, bool, string) {
	o, i, _ := p2pSwitch.NumPeers()
	numPeers := o + i
	listening := p2pSwitch.IsListening()
	network := config.App().GetString("Network")
	return numPeers, listening, network
}
