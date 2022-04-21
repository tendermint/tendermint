package server

import (
	"context"
	"net"

	"google.golang.org/grpc"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
)

type GRPCServer struct {
	service.BaseService
	logger log.Logger

	proto  string
	addr   string
	server *grpc.Server

	app types.Application
}

// NewGRPCServer returns a new gRPC ABCI server
func NewGRPCServer(logger log.Logger, protoAddr string, app types.Application) service.Service {
	proto, addr := tmnet.ProtocolAndAddress(protoAddr)
	s := &GRPCServer{
		logger: logger,
		proto:  proto,
		addr:   addr,
		app:    app,
	}
	s.BaseService = *service.NewBaseService(logger, "ABCIServer", s)
	return s
}

// OnStart starts the gRPC service.
func (s *GRPCServer) OnStart(ctx context.Context) error {
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}

	s.server = grpc.NewServer()
	types.RegisterABCIApplicationServer(s.server, &gRPCApplication{app: s.app})

	s.logger.Info("Listening", "proto", s.proto, "addr", s.addr)
	go func() {
		go func() {
			<-ctx.Done()
			s.server.GracefulStop()
		}()

		if err := s.server.Serve(ln); err != nil {
			s.logger.Error("error serving gRPC server", "err", err)
		}
	}()
	return nil
}

// OnStop stops the gRPC server.
func (s *GRPCServer) OnStop() { s.server.Stop() }

//-------------------------------------------------------

// gRPCApplication is a gRPC shim for Application
type gRPCApplication struct {
	app types.Application
}

func (app *gRPCApplication) Echo(_ context.Context, req *types.RequestEcho) (*types.ResponseEcho, error) {
	return &types.ResponseEcho{Message: req.Message}, nil
}

func (app *gRPCApplication) Flush(_ context.Context, req *types.RequestFlush) (*types.ResponseFlush, error) {
	return &types.ResponseFlush{}, nil
}

func (app *gRPCApplication) Info(ctx context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	return app.app.Info(ctx, *req)
}

func (app *gRPCApplication) CheckTx(ctx context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	return app.app.CheckTx(ctx, *req)
}

func (app *gRPCApplication) Query(ctx context.Context, req *types.RequestQuery) (*types.ResponseQuery, error) {
	return app.app.Query(ctx, *req)
}

func (app *gRPCApplication) Commit(ctx context.Context, req *types.RequestCommit) (*types.ResponseCommit, error) {
	return app.app.Commit(ctx)
}

func (app *gRPCApplication) InitChain(ctx context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	return app.app.InitChain(ctx, *req)
}

func (app *gRPCApplication) ListSnapshots(ctx context.Context, req *types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	return app.app.ListSnapshots(ctx, *req)
}

func (app *gRPCApplication) OfferSnapshot(ctx context.Context, req *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	return app.app.OfferSnapshot(ctx, *req)
}

func (app *gRPCApplication) LoadSnapshotChunk(ctx context.Context, req *types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	return app.app.LoadSnapshotChunk(ctx, *req)
}

func (app *gRPCApplication) ApplySnapshotChunk(ctx context.Context, req *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	return app.app.ApplySnapshotChunk(ctx, *req)
}

func (app *gRPCApplication) ExtendVote(ctx context.Context, req *types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	return app.app.ExtendVote(ctx, *req)
}

func (app *gRPCApplication) VerifyVoteExtension(ctx context.Context, req *types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	return app.app.VerifyVoteExtension(ctx, *req)
}

func (app *gRPCApplication) PrepareProposal(ctx context.Context, req *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	return app.app.PrepareProposal(ctx, *req)
}

func (app *gRPCApplication) ProcessProposal(ctx context.Context, req *types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	return app.app.ProcessProposal(ctx, *req)
}

func (app *gRPCApplication) FinalizeBlock(ctx context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	return app.app.FinalizeBlock(ctx, *req)
}
