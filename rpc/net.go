package rpc

import (
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
	"net/http"
)

//-----------------------------------------------------------------------------

// Request: {}

type ResponseStatus struct {
	ChainId           string
	LatestBlockHash   []byte
	LatestBlockHeight uint
	LatestBlockTime   int64 // nano
	Network           string
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	genesisHash := p2pSwitch.GetChainId()
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

	WriteAPIResponse(w, API_OK, ResponseStatus{genesisHash, latestBlockHash, latestHeight, latestBlockTime, config.App().GetString("Network")})
}

//-----------------------------------------------------------------------------

// Request: {}

type ResponseNetInfo struct {
	NumPeers  int
	Listening bool
	Network   string
}

func NetInfoHandler(w http.ResponseWriter, r *http.Request) {
	o, i, _ := p2pSwitch.NumPeers()
	numPeers := o + i
	listening := p2pSwitch.IsListening()
	network := config.App().GetString("Network")
	WriteAPIResponse(w, API_OK,
		ResponseNetInfo{numPeers, listening, network})
}
