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
	conns      map[int]net.Conn
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
		conns:      make(map[int]net.Conn),
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

	for id := range s.conns {
		if err := s.unsafeRmConn(id); err != nil {
			s.logger.Error("error closing connection", "id", id, "conn", conn, "err", err)
		}
	}
}

func (s *SocketServer) addConn(conn net.Conn, closer func()) int {
	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()

	connID := s.nextConnID
	s.nextConnID++
	s.conns[connID] = conn
	s.connsClose[connID] = closer
	return connID
}

// deletes conn even if close errs
func (s *SocketServer) rmConn(connID int) error {
	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()
	return s.unsafeRmConn(connID)
}

func (s *SocketServer) unsafeRmConn(connID int) error {
	if closer, ok := s.connsClose[connID]; ok {
		closer()
		delete(s.connsClose, connID)
	}

	conn, ok := s.conns[connID]
	if !ok {
		return nil
	}

	delete(s.conns, connID)
	return conn.Close()
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
		connID := s.addConn(conn, ccancel)

		s.logger.Info("Accepted a new connection", "id", connID)

		responses := make(chan *types.Response, 1000) // A channel to buffer responses

		once := &sync.Once{}
		closer := func(err error) {
			ccancel()
			once.Do(func() {
				if rcerr := s.rmConn(connID); rcerr != nil {
					s.logger.Error("error closing connection",
						"id", connID,
						"close_err", rcerr,
						"err", err)
				}

				switch {
				case errors.Is(err, io.EOF):
					s.logger.Error("Connection was closed by client", "id", connID)
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
		r := recover()
		if r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			closer(fmt.Errorf("recovered from panic: %v\n%s", r, buf))
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}

		var req = &types.Request{}
		err := types.ReadMessage(bufReader, req)
		if err != nil {
			closer(fmt.Errorf("error reading message: %w", err))
			return
		}
		s.handleRequest(ctx, req, responses)
	}
}

func (s *SocketServer) handleRequest(ctx context.Context, req *types.Request, responses chan<- *types.Response) {
	var resp *types.Response

	switch r := req.Value.(type) {
	case *types.Request_Echo:
		resp = types.ToResponseEcho(r.Echo.Message)
	case *types.Request_Flush:
		resp = types.ToResponseFlush()
	case *types.Request_Info:
		resp = types.ToResponseInfo(s.app.Info(*r.Info))
	case *types.Request_CheckTx:
		resp = types.ToResponseCheckTx(s.app.CheckTx(*r.CheckTx))
	case *types.Request_Commit:
		resp = types.ToResponseCommit(s.app.Commit())
	case *types.Request_Query:
		resp = types.ToResponseQuery(s.app.Query(*r.Query))
	case *types.Request_InitChain:
		resp = types.ToResponseInitChain(s.app.InitChain(*r.InitChain))
	case *types.Request_ListSnapshots:
		resp = types.ToResponseListSnapshots(s.app.ListSnapshots(*r.ListSnapshots))
	case *types.Request_OfferSnapshot:
		resp = types.ToResponseOfferSnapshot(s.app.OfferSnapshot(*r.OfferSnapshot))
	case *types.Request_PrepareProposal:
		resp = types.ToResponsePrepareProposal(s.app.PrepareProposal(*r.PrepareProposal))
	case *types.Request_ProcessProposal:
		resp = types.ToResponseProcessProposal(s.app.ProcessProposal(*r.ProcessProposal))
	case *types.Request_LoadSnapshotChunk:
		resp = types.ToResponseLoadSnapshotChunk(s.app.LoadSnapshotChunk(*r.LoadSnapshotChunk))
	case *types.Request_ApplySnapshotChunk:
		resp = types.ToResponseApplySnapshotChunk(s.app.ApplySnapshotChunk(*r.ApplySnapshotChunk))
	case *types.Request_ExtendVote:
		resp = types.ToResponseExtendVote(s.app.ExtendVote(*r.ExtendVote))
	case *types.Request_VerifyVoteExtension:
		resp = types.ToResponseVerifyVoteExtension(s.app.VerifyVoteExtension(*r.VerifyVoteExtension))
	case *types.Request_FinalizeBlock:
		resp = types.ToResponseFinalizeBlock(s.app.FinalizeBlock(*r.FinalizeBlock))
	default:
		resp = types.ToResponseException("Unknown request")
	}

	select {
	case <-ctx.Done():
	case responses <- resp:
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
