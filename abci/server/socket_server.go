package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
)

// var maxNumberConnections = 2

type SocketServer struct {
	service.BaseService
	logger log.Logger

	proto    string
	addr     string
	listener net.Listener

	connsMtx   sync.Mutex
	connsClose map[int]func()
	nextConnID int

	app types.Application
}

func NewSocketServer(logger log.Logger, protoAddr string, app types.Application) service.Service {
	proto, addr := tmnet.ProtocolAndAddress(protoAddr)
	s := &SocketServer{
		logger:     logger,
		proto:      proto,
		addr:       addr,
		listener:   nil,
		app:        app,
		connsClose: make(map[int]func()),
	}
	s.BaseService = *service.NewBaseService(logger, "ABCIServer", s)
	return s
}

func (s *SocketServer) OnStart(ctx context.Context) error {
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}

	s.listener = ln
	go s.acceptConnectionsRoutine(ctx)

	return nil
}

func (s *SocketServer) OnStop() {
	if err := s.listener.Close(); err != nil {
		s.logger.Error("error closing listener", "err", err)
	}

	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()

	for _, closer := range s.connsClose {
		closer()
	}
}

func (s *SocketServer) addConn(closer func()) int {
	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()

	connID := s.nextConnID
	s.nextConnID++
	s.connsClose[connID] = closer
	return connID
}

// deletes conn even if close errs
func (s *SocketServer) rmConn(connID int) {
	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()
	if closer, ok := s.connsClose[connID]; ok {
		closer()
		delete(s.connsClose, connID)
	}
}

func (s *SocketServer) acceptConnectionsRoutine(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		// Accept a connection
		s.logger.Info("Waiting for new connection...")
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.IsRunning() {
				return // Ignore error from listener closing.
			}
			s.logger.Error("Failed to accept connection", "err", err)
			continue
		}

		cctx, ccancel := context.WithCancel(ctx)
		connID := s.addConn(ccancel)

		s.logger.Info("Accepted a new connection", "id", connID)

		responses := make(chan *types.Response, 1000) // A channel to buffer responses

		once := &sync.Once{}
		closer := func(err error) {
			ccancel()
			once.Do(func() {
				if cerr := conn.Close(); err != nil {
					s.logger.Error("error closing connection",
						"id", connID,
						"close_err", cerr,
						"err", err)
				}
				s.rmConn(connID)

				switch {
				case errors.Is(err, context.Canceled):
					s.logger.Error("Connection terminated",
						"id", connID,
						"err", err)
				case errors.Is(err, context.DeadlineExceeded):
					s.logger.Error("Connection encountered timeout",
						"id", connID,
						"err", err)
				case errors.Is(err, io.EOF):
					s.logger.Error("Connection was closed by client",
						"id", connID)
				case err != nil:
					s.logger.Error("Connection error",
						"id", connID,
						"err", err)
				default:
					s.logger.Error("Connection was closed",
						"id", connID)
				}
			})
		}

		// Read requests from conn and deal with them
		go s.handleRequests(cctx, closer, conn, responses)
		// Pull responses from 'responses' and write them to conn.
		go s.handleResponses(cctx, closer, conn, responses)
	}
}

// Read requests from conn and deal with them
func (s *SocketServer) handleRequests(
	ctx context.Context,
	closer func(error),
	conn io.Reader,
	responses chan<- *types.Response,
) {
	var bufReader = bufio.NewReader(conn)

	defer func() {
		// make sure to recover from any app-related panics to allow proper socket cleanup
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			closer(fmt.Errorf("recovered from panic: %v\n%s", r, buf))
		}
	}()

	for {
		req := &types.Request{}
		if err := types.ReadMessage(bufReader, req); err != nil {
			closer(fmt.Errorf("error reading message: %w", err))
			return
		}

		resp := s.processRequest(req)
		select {
		case <-ctx.Done():
			closer(ctx.Err())
			return
		case responses <- resp:
		}
	}
}

func (s *SocketServer) processRequest(req *types.Request) *types.Response {
	switch r := req.Value.(type) {
	case *types.Request_Echo:
		return types.ToResponseEcho(r.Echo.Message)
	case *types.Request_Flush:
		return types.ToResponseFlush()
	case *types.Request_Info:
		return types.ToResponseInfo(s.app.Info(*r.Info))
	case *types.Request_CheckTx:
		return types.ToResponseCheckTx(s.app.CheckTx(*r.CheckTx))
	case *types.Request_Commit:
		return types.ToResponseCommit(s.app.Commit())
	case *types.Request_Query:
		return types.ToResponseQuery(s.app.Query(*r.Query))
	case *types.Request_InitChain:
		return types.ToResponseInitChain(s.app.InitChain(*r.InitChain))
	case *types.Request_ListSnapshots:
		return types.ToResponseListSnapshots(s.app.ListSnapshots(*r.ListSnapshots))
	case *types.Request_OfferSnapshot:
		return types.ToResponseOfferSnapshot(s.app.OfferSnapshot(*r.OfferSnapshot))
	case *types.Request_PrepareProposal:
		return types.ToResponsePrepareProposal(s.app.PrepareProposal(*r.PrepareProposal))
	case *types.Request_ProcessProposal:
		return types.ToResponseProcessProposal(s.app.ProcessProposal(*r.ProcessProposal))
	case *types.Request_LoadSnapshotChunk:
		return types.ToResponseLoadSnapshotChunk(s.app.LoadSnapshotChunk(*r.LoadSnapshotChunk))
	case *types.Request_ApplySnapshotChunk:
		return types.ToResponseApplySnapshotChunk(s.app.ApplySnapshotChunk(*r.ApplySnapshotChunk))
	case *types.Request_ExtendVote:
		return types.ToResponseExtendVote(s.app.ExtendVote(*r.ExtendVote))
	case *types.Request_VerifyVoteExtension:
		return types.ToResponseVerifyVoteExtension(s.app.VerifyVoteExtension(*r.VerifyVoteExtension))
	case *types.Request_FinalizeBlock:
		return types.ToResponseFinalizeBlock(s.app.FinalizeBlock(*r.FinalizeBlock))
	default:
		return types.ToResponseException("Unknown request")
	}
}

// Pull responses from 'responses' and write them to conn.
func (s *SocketServer) handleResponses(
	ctx context.Context,
	closer func(error),
	conn io.Writer,
	responses <-chan *types.Response,
) {
	bw := bufio.NewWriter(conn)
	for {
		select {
		case <-ctx.Done():
			closer(ctx.Err())
			return
		case res := <-responses:
			if err := types.WriteMessage(res, bw); err != nil {
				closer(fmt.Errorf("error writing message: %w", err))
				return
			}
			if err := bw.Flush(); err != nil {
				closer(fmt.Errorf("error writing message: %w", err))
				return
			}
		}
	}
}
