package proxy

import (
	"context"
	"time"

	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

// proxyService wraps a light RPC client to export the RPC service interfaces.
// This is needed because the service and the client use different signatures
// for some of the methods.
type proxyService struct {
	*lrpc.Client
}

func (p proxyService) ABCIQuery(ctx context.Context, path string, data tmbytes.HexBytes, height int64, prove bool) (*coretypes.ResultABCIQuery, error) {
	return p.ABCIQueryWithOptions(ctx, path, data, rpcclient.ABCIQueryOptions{
		Height: height,
		Prove:  prove,
	})
}

func (p proxyService) GetConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	return p.ConsensusState(ctx)
}

func (p proxyService) Events(ctx context.Context,
	filter *coretypes.EventFilter,
	maxItems int,
	before, after cursor.Cursor,
	waitTime time.Duration,
) (*coretypes.ResultEvents, error) {
	return p.Client.Events(ctx, &coretypes.RequestEvents{
		Filter:   filter,
		MaxItems: maxItems,
		Before:   before.String(),
		After:    after.String(),
		WaitTime: waitTime,
	})
}

func (p proxyService) Subscribe(ctx context.Context, query string) (*coretypes.ResultSubscribe, error) {
	return p.SubscribeWS(ctx, query)
}

func (p proxyService) Unsubscribe(ctx context.Context, query string) (*coretypes.ResultUnsubscribe, error) {
	return p.UnsubscribeWS(ctx, query)
}

func (p proxyService) UnsubscribeAll(ctx context.Context) (*coretypes.ResultUnsubscribe, error) {
	return p.UnsubscribeAllWS(ctx)
}

func (p proxyService) BroadcastEvidence(ctx context.Context, ev coretypes.Evidence) (*coretypes.ResultBroadcastEvidence, error) {
	return p.Client.BroadcastEvidence(ctx, ev.Value)
}
