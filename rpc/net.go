package rpc

import (
	"github.com/tendermint/tendermint/config"
	"net/http"
)

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	genesisHash := blockStore.LoadBlockMeta(0).Hash
	latestHeight := blockStore.Height()
	latestBlockMeta := blockStore.LoadBlockMeta(latestHeight)
	latestBlockHash := latestBlockMeta.Hash
	latestBlockTime := latestBlockMeta.Header.Time.UnixNano()
	WriteAPIResponse(w, API_OK, struct {
		GenesisHash       []byte
		LatestBlockHash   []byte
		LatestBlockHeight uint
		LatestBlockTime   int64 // nano
		Network           string
	}{genesisHash, latestBlockHash, latestHeight, latestBlockTime, config.App().GetString("Network")})
}

func NetInfoHandler(w http.ResponseWriter, r *http.Request) {
	o, i, _ := p2pSwitch.NumPeers()
	numPeers := o + i
	listening := p2pSwitch.IsListening()
	network := config.App().GetString("Network")
	WriteAPIResponse(w, API_OK, struct {
		NumPeers  int
		Listening bool
		Network   string
	}{numPeers, listening, network})
}
