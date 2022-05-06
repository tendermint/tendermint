package privval

import (
	"context"
	"io"
	"sync"

	"github.com/tendermint/tendermint/libs/service"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

// ValidationRequestHandlerFunc handles different remoteSigner requests
type ValidationRequestHandlerFunc func(
	ctx context.Context,
	privVal types.PrivValidator,
	requestMessage privvalproto.Message,
	chainID string) (privvalproto.Message, error)

type SignerServer struct {
	service.BaseService

	endpoint *SignerDialerEndpoint
	chainID  string
	privVal  types.PrivValidator

	handlerMtx               sync.Mutex
	validationRequestHandler ValidationRequestHandlerFunc
}

func NewSignerServer(endpoint *SignerDialerEndpoint, chainID string, privVal types.PrivValidator) *SignerServer {
	ss := &SignerServer{
		endpoint:                 endpoint,
		chainID:                  chainID,
		privVal:                  privVal,
		validationRequestHandler: DefaultValidationRequestHandler,
	}

	ss.BaseService = *service.NewBaseService(endpoint.logger, "SignerServer", ss)

	return ss
}

// OnStart implements service.Service.
func (ss *SignerServer) OnStart(ctx context.Context) error {
	go ss.serviceLoop(ctx)
	return nil
}

// OnStop implements service.Service.
func (ss *SignerServer) OnStop() {
	ss.endpoint.logger.Debug("SignerServer: OnStop calling Close")
	_ = ss.endpoint.Close()
}

// SetRequestHandler override the default function that is used to service requests
func (ss *SignerServer) SetRequestHandler(validationRequestHandler ValidationRequestHandlerFunc) {
	ss.handlerMtx.Lock()
	defer ss.handlerMtx.Unlock()
	ss.validationRequestHandler = validationRequestHandler
}

func (ss *SignerServer) servicePendingRequest(ctx context.Context) {
	if !ss.IsRunning() {
		return // Ignore error from closing.
	}

	req, err := ss.endpoint.ReadMessage()
	if err != nil {
		if err != io.EOF {
			ss.endpoint.logger.Error("SignerServer: HandleMessage", "err", err)
		}
		return
	}

	var res privvalproto.Message
	{
		// limit the scope of the lock
		ss.handlerMtx.Lock()
		defer ss.handlerMtx.Unlock()
		res, err = ss.validationRequestHandler(ctx, ss.privVal, req, ss.chainID) // todo
		if err != nil {
			// only log the error; we'll reply with an error in res
			ss.endpoint.logger.Error("SignerServer: handleMessage", "err", err)
		}
	}

	err = ss.endpoint.WriteMessage(res)
	if err != nil {
		ss.endpoint.logger.Error("SignerServer: writeMessage", "err", err)
	}
}

func (ss *SignerServer) serviceLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := ss.endpoint.ensureConnection(ctx); err != nil {
				return
			}
			ss.servicePendingRequest(ctx)
		}
	}
}
