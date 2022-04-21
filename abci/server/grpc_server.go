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

func (app *gRPCApplication) Info(_ context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	res := app.app.Info(*req)
	return &res, nil
}

func (app *gRPCApplication) CheckTx(_ context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	res := app.app.CheckTx(*req)
	return &res, nil
}

func (app *gRPCApplication) Query(_ context.Context, req *types.RequestQuery) (*types.ResponseQuery, error) {
	res := app.app.Query(*req)
	return &res, nil
}

func (app *gRPCApplication) Commit(_ context.Context, req *types.RequestCommit) (*types.ResponseCommit, error) {
	res := app.app.Commit()
	return &res, nil
}

func (app *gRPCApplication) InitChain(_ context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	res := app.app.InitChain(*req)
	return &res, nil
}

func (app *gRPCApplication) ListSnapshots(_ context.Context, req *types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
	res := app.app.ListSnapshots(*req)
	return &res, nil
}

func (app *gRPCApplication) OfferSnapshot(_ context.Context, req *types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
	res := app.app.OfferSnapshot(*req)
	return &res, nil
}

func (app *gRPCApplication) LoadSnapshotChunk(_ context.Context, req *types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
	res := app.app.LoadSnapshotChunk(*req)
	return &res, nil
}

func (app *gRPCApplication) ApplySnapshotChunk(_ context.Context, req *types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
	res := app.app.ApplySnapshotChunk(*req)
	return &res, nil
}

func (app *gRPCApplication) ExtendVote(_ context.Context, req *types.RequestExtendVote) (*types.ResponseExtendVote, error) {
	res := app.app.ExtendVote(*req)
	return &res, nil
}

func (app *gRPCApplication) VerifyVoteExtension(_ context.Context, req *types.RequestVerifyVoteExtension) (*types.ResponseVerifyVoteExtension, error) {
	res := app.app.VerifyVoteExtension(*req)
	return &res, nil
}

func (app *gRPCApplication) PrepareProposal(_ context.Context, req *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	res := app.app.PrepareProposal(*req)
	return &res, nil
}

func (app *gRPCApplication) ProcessProposal(_ context.Context, req *types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
	res := app.app.ProcessProposal(*req)
	return &res, nil
}

func (app *gRPCApplication) FinalizeBlock(_ context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	res := app.app.FinalizeBlock(*req)
	return &res, nil
}
