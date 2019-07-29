package privval

import (
	"io"
	"sync"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
)

// ValidationRequestHandlerFunc handles different remoteSigner requests
type ValidationRequestHandlerFunc func(
	privVal types.PrivValidator,
	requestMessage SignerMessage,
	chainID string) (SignerMessage, error)

type SignerServer struct {
	cmn.BaseService

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

	ss.BaseService = *cmn.NewBaseService(endpoint.Logger, "SignerServer", ss)

	return ss
}

// OnStart implements cmn.Service.
func (ss *SignerServer) OnStart() error {
	go ss.serviceLoop()
	return nil
}

// OnStop implements cmn.Service.
func (ss *SignerServer) OnStop() {
	ss.endpoint.Logger.Debug("SignerServer: OnStop calling Close")
	_ = ss.endpoint.Close()
}

// SetRequestHandler override the default function that is used to service requests
func (ss *SignerServer) SetRequestHandler(validationRequestHandler ValidationRequestHandlerFunc) {
	ss.handlerMtx.Lock()
	defer ss.handlerMtx.Unlock()
	ss.validationRequestHandler = validationRequestHandler
}

func (ss *SignerServer) servicePendingRequest() {
	if !ss.IsRunning() {
		return // Ignore error from closing.
	}

	req, err := ss.endpoint.ReadMessage()
	if err != nil {
		if err != io.EOF {
			ss.Logger.Error("SignerServer: HandleMessage", "err", err)
		}
		return
	}

	var res SignerMessage
	{
		// limit the scope of the lock
		ss.handlerMtx.Lock()
		defer ss.handlerMtx.Unlock()
		res, err = ss.validationRequestHandler(ss.privVal, req, ss.chainID)
		if err != nil {
			// only log the error; we'll reply with an error in res
			ss.Logger.Error("SignerServer: handleMessage", "err", err)
		}
	}

	if res != nil {
		err = ss.endpoint.WriteMessage(res)
		if err != nil {
			ss.Logger.Error("SignerServer: writeMessage", "err", err)
		}
	}
}

func (ss *SignerServer) serviceLoop() {
	for {
		select {
		default:
			err := ss.endpoint.ensureConnection()
			if err != nil {
				return
			}
			ss.servicePendingRequest()

		case <-ss.Quit():
			return
		}
	}
}
