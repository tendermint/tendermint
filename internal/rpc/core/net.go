package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// NetInfo returns network info.
// More: https://docs.tendermint.com/master/rpc/#/Info/net_info
func (env *Environment) NetInfo(ctx *rpctypes.Context) (*coretypes.ResultNetInfo, error) {
	var peers []coretypes.Peer

	peerList := env.PeerManager.Peers()
	for _, peer := range peerList {
		addrs := env.PeerManager.Addresses(peer)
		if len(addrs) == 0 {
			continue
		}

		peers = append(peers, coretypes.Peer{
			ID:  peer,
			URL: addrs[0].String(),
		})
	}

	return &coretypes.ResultNetInfo{
		Listening: env.P2PTransport.IsListening(),
		Listeners: env.P2PTransport.Listeners(),
		NPeers:    len(peers),
		Peers:     peers,
	}, nil
}

// Genesis returns genesis file.
// More: https://docs.tendermint.com/master/rpc/#/Info/genesis
func (env *Environment) Genesis(ctx *rpctypes.Context) (*coretypes.ResultGenesis, error) {
	if len(env.genChunks) > 1 {
		return nil, errors.New("genesis response is large, please use the genesis_chunked API instead")
	}

	return &coretypes.ResultGenesis{Genesis: env.GenDoc}, nil
}

func (env *Environment) GenesisChunked(ctx *rpctypes.Context, chunk uint) (*coretypes.ResultGenesisChunk, error) {
	if env.genChunks == nil {
		return nil, fmt.Errorf("service configuration error, genesis chunks are not initialized")
	}

	if len(env.genChunks) == 0 {
		return nil, fmt.Errorf("service configuration error, there are no chunks")
	}

	id := int(chunk)

	if id > len(env.genChunks)-1 {
		return nil, fmt.Errorf("there are %d chunks, %d is invalid", len(env.genChunks)-1, id)
	}

	return &coretypes.ResultGenesisChunk{
		TotalChunks: len(env.genChunks),
		ChunkNumber: id,
		Data:        env.genChunks[id],
	}, nil
}

func getIDs(peers []string) ([]string, error) {
	ids := make([]string, 0, len(peers))

	for _, peer := range peers {

		spl := strings.Split(peer, "@")
		if len(spl) != 2 {
			return nil, p2p.ErrNetAddressNoID{Addr: peer}
		}
		ids = append(ids, spl[0])

	}
	return ids, nil
}
