package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/rpc/coretypes"
)

// NetInfo returns network info.
// More: https://docs.tendermint.com/master/rpc/#/Info/net_info
func (env *Environment) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	peerList := env.PeerManager.Peers()

	peers := make([]coretypes.Peer, 0, len(peerList))
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
		Listening: env.IsListening,
		Listeners: env.Listeners,
		NPeers:    len(peers),
		Peers:     peers,
	}, nil
}

// Genesis returns genesis file.
// More: https://docs.tendermint.com/master/rpc/#/Info/genesis
func (env *Environment) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	if len(env.genChunks) > 1 {
		return nil, errors.New("genesis response is large, please use the genesis_chunked API instead")
	}

	return &coretypes.ResultGenesis{Genesis: env.GenDoc}, nil
}

func (env *Environment) GenesisChunked(ctx context.Context, req *coretypes.RequestGenesisChunked) (*coretypes.ResultGenesisChunk, error) {
	if env.genChunks == nil {
		return nil, fmt.Errorf("service configuration error, genesis chunks are not initialized")
	}

	if len(env.genChunks) == 0 {
		return nil, fmt.Errorf("service configuration error, there are no chunks")
	}

	id := int(req.Chunk)

	if id > len(env.genChunks)-1 {
		return nil, fmt.Errorf("there are %d chunks, %d is invalid", len(env.genChunks)-1, id)
	}

	return &coretypes.ResultGenesisChunk{
		TotalChunks: len(env.genChunks),
		ChunkNumber: id,
		Data:        env.genChunks[id],
	}, nil
}
