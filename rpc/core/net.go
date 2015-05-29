package core

import (
	"io/ioutil"

	dbm "github.com/tendermint/tendermint/db"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

func Status() (*ctypes.ResponseStatus, error) {
	db := dbm.NewMemDB()
	genesisState := sm.MakeGenesisStateFromFile(db, config.GetString("genesis_file"))
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

	return &ctypes.ResponseStatus{
		Moniker:           config.GetString("moniker"),
		ChainID:           config.GetString("chain_id"),
		Version:           config.GetString("version"),
		GenesisHash:       genesisHash,
		PubKey:            privValidator.PubKey,
		LatestBlockHash:   latestBlockHash,
		LatestBlockHeight: latestHeight,
		LatestBlockTime:   latestBlockTime}, nil
}

//-----------------------------------------------------------------------------

func NetInfo() (*ctypes.ResponseNetInfo, error) {
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
	return &ctypes.ResponseNetInfo{
		Listening: listening,
		Listeners: listeners,
		Peers:     peers,
	}, nil
}

//-----------------------------------------------------------------------------

// returns pointer because the rpc-gen code returns nil (TODO!)
func Genesis() (*string, error) {
	b, err := ioutil.ReadFile(config.GetString("genesis_file"))
	if err != nil {
		return nil, err
	}
	ret := string(b)
	return &ret, nil
}
