package abciclient

import (
	"context"
	"fmt"
	"sync"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

const (
	dialRetryIntervalSeconds = 3
	echoRetryIntervalSeconds = 1
)

//go:generate ../../scripts/mockery_generate.sh Client

// Client defines the interface for an ABCI client.
//
// NOTE these are client errors, eg. ABCI socket connectivity issues.
// Application-related errors are reflected in response via ABCI error codes
// and (potentially) error response.
type Client interface {
	service.Service
	types.Application

	Error() error
	Flush(context.Context) error
	Echo(context.Context, string) (*types.ResponseEcho, error)
}

//----------------------------------------

// NewClient returns a new ABCI client of the specified transport type.
// It returns an error if the transport is not "socket" or "grpc"
func NewClient(logger log.Logger, addr, transport string, mustConnect bool) (Client, error) {
	switch transport {
	case "socket":
		return NewSocketClient(logger, addr, mustConnect), nil
	case "grpc":
		return NewGRPCClient(logger, addr, mustConnect), nil
	default:
		return nil, fmt.Errorf("unknown abci transport %s", transport)
	}
}

type requestAndResponse struct {
	*types.Request
	*types.Response

	mtx    sync.Mutex
	signal chan struct{}
}

func makeReqRes(req *types.Request) *requestAndResponse {
	return &requestAndResponse{
		Request:  req,
		Response: nil,
		signal:   make(chan struct{}),
	}
}

// markDone marks the ReqRes object as done.
func (r *requestAndResponse) markDone() {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	close(r.signal)
}
