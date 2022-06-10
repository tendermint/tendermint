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
func (s *SocketServer) handleRequests(ctx context.Context, closer func(error), conn io.Reader, responses chan<- *types.Response) {
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

		resp, err := s.processRequest(ctx, req)
		if err != nil {
			closer(err)
			return
		}

		select {
		case <-ctx.Done():
			closer(ctx.Err())
			return
		case responses <- resp:
		}
	}
}

func (s *SocketServer) processRequest(ctx context.Context, req *types.Request) (*types.Response, error) {
	switch r := req.Value.(type) {
	case *types.Request_Echo:
		return types.ToResponseEcho(r.Echo.Message), nil
	case *types.Request_Flush:
		return types.ToResponseFlush(), nil
	case *types.Request_Info:
		res, err := s.app.Info(ctx, r.Info)
		if err != nil {
			return nil, err
		}

		return types.ToResponseInfo(res), nil
	case *types.Request_CheckTx:
		res, err := s.app.CheckTx(ctx, r.CheckTx)
		if err != nil {
			return nil, err
		}
		return types.ToResponseCheckTx(res), nil
	case *types.Request_Commit:
		res, err := s.app.Commit(ctx)
		if err != nil {
			return nil, err
		}
		return types.ToResponseCommit(res), nil
	case *types.Request_Query:
		res, err := s.app.Query(ctx, r.Query)
		if err != nil {
			return nil, err
		}
		return types.ToResponseQuery(res), nil
	case *types.Request_InitChain:
		res, err := s.app.InitChain(ctx, r.InitChain)
		if err != nil {
			return nil, err
		}
		return types.ToResponseInitChain(res), nil
	case *types.Request_ListSnapshots:
		res, err := s.app.ListSnapshots(ctx, r.ListSnapshots)
		if err != nil {
			return nil, err
		}
		return types.ToResponseListSnapshots(res), nil
	case *types.Request_OfferSnapshot:
		res, err := s.app.OfferSnapshot(ctx, r.OfferSnapshot)
		if err != nil {
			return nil, err
		}
		return types.ToResponseOfferSnapshot(res), nil
	case *types.Request_PrepareProposal:
		res, err := s.app.PrepareProposal(ctx, r.PrepareProposal)
		if err != nil {
			return nil, err
		}
		return types.ToResponsePrepareProposal(res), nil
	case *types.Request_ProcessProposal:
		res, err := s.app.ProcessProposal(ctx, r.ProcessProposal)
		if err != nil {
			return nil, err
		}
		return types.ToResponseProcessProposal(res), nil
	case *types.Request_LoadSnapshotChunk:
		res, err := s.app.LoadSnapshotChunk(ctx, r.LoadSnapshotChunk)
		if err != nil {
			return nil, err
		}
		return types.ToResponseLoadSnapshotChunk(res), nil
	case *types.Request_ApplySnapshotChunk:
		res, err := s.app.ApplySnapshotChunk(ctx, r.ApplySnapshotChunk)
		if err != nil {
			return nil, err
		}
		return types.ToResponseApplySnapshotChunk(res), nil
	case *types.Request_ExtendVote:
		res, err := s.app.ExtendVote(ctx, r.ExtendVote)
		if err != nil {
			return nil, err
		}
		return types.ToResponseExtendVote(res), nil
	case *types.Request_VerifyVoteExtension:
		res, err := s.app.VerifyVoteExtension(ctx, r.VerifyVoteExtension)
		if err != nil {
			return nil, err
		}
		return types.ToResponseVerifyVoteExtension(res), nil
	case *types.Request_FinalizeBlock:
		res, err := s.app.FinalizeBlock(ctx, r.FinalizeBlock)
		if err != nil {
			return nil, err
		}
		return types.ToResponseFinalizeBlock(res), nil
	default:
		return types.ToResponseException("Unknown request"), errors.New("unknown request type")
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
