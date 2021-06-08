package server

import (
	"bufio"
	"context"
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
	nextConnID int

	appMtx sync.Mutex
	app    types.Application
}

func NewSocketServer(logger log.Logger, protoAddr string, app types.Application) service.Service {
	proto, addr := tmnet.ProtocolAndAddress(protoAddr)
	s := &SocketServer{
		logger:   logger,
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
		conns:    make(map[int]net.Conn),
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

	for id, conn := range s.conns {
		delete(s.conns, id)
		if err := conn.Close(); err != nil {
			s.logger.Error("error closing connection", "id", id, "conn", conn, "err", err)
		}
	}
}

func (s *SocketServer) addConn(conn net.Conn) int {
	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()

	connID := s.nextConnID
	s.nextConnID++
	s.conns[connID] = conn

	return connID
}

// deletes conn even if close errs
func (s *SocketServer) rmConn(connID int) error {
	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()

	conn, ok := s.conns[connID]
	if !ok {
		return fmt.Errorf("connection %d does not exist", connID)
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

		s.logger.Info("Accepted a new connection")

		connID := s.addConn(conn)

		closeConn := make(chan error, 2)              // Push to signal connection closed
		responses := make(chan *types.Response, 1000) // A channel to buffer responses

		// Read requests from conn and deal with them
		go s.handleRequests(ctx, closeConn, conn, responses)
		// Pull responses from 'responses' and write them to conn.
		go s.handleResponses(ctx, closeConn, conn, responses)

		// Wait until signal to close connection
		go s.waitForClose(ctx, closeConn, connID)
	}
}

func (s *SocketServer) waitForClose(ctx context.Context, closeConn chan error, connID int) {
	defer func() {
		// Close the connection
		if err := s.rmConn(connID); err != nil {
			s.logger.Error("error closing connection", "err", err)
		}
	}()

	select {
	case <-ctx.Done():
		return
	case err := <-closeConn:
		switch {
		case err == io.EOF:
			s.logger.Error("Connection was closed by client")
		case err != nil:
			s.logger.Error("Connection error", "err", err)
		default:
			// never happens
			s.logger.Error("Connection was closed")
		}
	}
}

// Read requests from conn and deal with them
func (s *SocketServer) handleRequests(
	ctx context.Context,
	closeConn chan error,
	conn io.Reader,
	responses chan<- *types.Response,
) {
	var count int
	var bufReader = bufio.NewReader(conn)

	defer func() {
		// make sure to recover from any app-related panics to allow proper socket cleanup
		r := recover()
		if r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			err := fmt.Errorf("recovered from panic: %v\n%s", r, buf)
			closeConn <- err
			s.appMtx.Unlock()
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}

		var req = &types.Request{}
		err := types.ReadMessage(bufReader, req)
		if err != nil {
			if err == io.EOF {
				closeConn <- err
			} else {
				closeConn <- fmt.Errorf("error reading message: %w", err)
			}
			return
		}
		s.appMtx.Lock()
		count++
		s.handleRequest(req, responses)
		s.appMtx.Unlock()
	}
}

func (s *SocketServer) handleRequest(req *types.Request, responses chan<- *types.Response) {
	switch r := req.Value.(type) {
	case *types.Request_Echo:
		responses <- types.ToResponseEcho(r.Echo.Message)
	case *types.Request_Flush:
		responses <- types.ToResponseFlush()
	case *types.Request_Info:
		res := s.app.Info(*r.Info)
		responses <- types.ToResponseInfo(res)
	case *types.Request_CheckTx:
		res := s.app.CheckTx(*r.CheckTx)
		responses <- types.ToResponseCheckTx(res)
	case *types.Request_Commit:
		res := s.app.Commit()
		responses <- types.ToResponseCommit(res)
	case *types.Request_Query:
		res := s.app.Query(*r.Query)
		responses <- types.ToResponseQuery(res)
	case *types.Request_InitChain:
		res := s.app.InitChain(*r.InitChain)
		responses <- types.ToResponseInitChain(res)
	case *types.Request_ListSnapshots:
		res := s.app.ListSnapshots(*r.ListSnapshots)
		responses <- types.ToResponseListSnapshots(res)
	case *types.Request_OfferSnapshot:
		res := s.app.OfferSnapshot(*r.OfferSnapshot)
		responses <- types.ToResponseOfferSnapshot(res)
	case *types.Request_PrepareProposal:
		res := s.app.PrepareProposal(*r.PrepareProposal)
		responses <- types.ToResponsePrepareProposal(res)
	case *types.Request_ProcessProposal:
		res := s.app.ProcessProposal(*r.ProcessProposal)
		responses <- types.ToResponseProcessProposal(res)
	case *types.Request_LoadSnapshotChunk:
		res := s.app.LoadSnapshotChunk(*r.LoadSnapshotChunk)
		responses <- types.ToResponseLoadSnapshotChunk(res)
	case *types.Request_ApplySnapshotChunk:
		res := s.app.ApplySnapshotChunk(*r.ApplySnapshotChunk)
		responses <- types.ToResponseApplySnapshotChunk(res)
	case *types.Request_ExtendVote:
		res := s.app.ExtendVote(*r.ExtendVote)
		responses <- types.ToResponseExtendVote(res)
	case *types.Request_VerifyVoteExtension:
		res := s.app.VerifyVoteExtension(*r.VerifyVoteExtension)
		responses <- types.ToResponseVerifyVoteExtension(res)
	case *types.Request_FinalizeBlock:
		res := s.app.FinalizeBlock(*r.FinalizeBlock)
		responses <- types.ToResponseFinalizeBlock(res)
	default:
		responses <- types.ToResponseException("Unknown request")
	}
}

// Pull responses from 'responses' and write them to conn.
func (s *SocketServer) handleResponses(
	ctx context.Context,
	closeConn chan error,
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
				select {
				case <-ctx.Done():
				case closeConn <- fmt.Errorf("error writing message: %w", err):
				}
				return
			}
			if err := bw.Flush(); err != nil {
				select {
				case <-ctx.Done():
				case closeConn <- fmt.Errorf("error flushing write buffer: %w", err):
				}

				return
			}
		}
	}
}
