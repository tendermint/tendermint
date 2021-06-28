package rpctest

import (
	"context"
	"fmt"
	"os"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/proxy"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	core_grpc "github.com/tendermint/tendermint/rpc/grpc"
	rpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

// Options helps with specifying some parameters for our RPC testing for greater
// control.
type Options struct {
	suppressStdout bool
}

func waitForRPC(ctx context.Context, conf *cfg.Config) {
	laddr := conf.RPC.ListenAddress
	client, err := rpcclient.New(laddr)
	if err != nil {
		panic(err)
	}
	result := new(ctypes.ResultStatus)
	for {
		_, err := client.Call(ctx, "status", map[string]interface{}{}, result)
		if err == nil {
			return
		}

		fmt.Println("error", err)
		time.Sleep(time.Millisecond)
	}
}

func waitForGRPC(ctx context.Context, conf *cfg.Config) {
	client := GetGRPCClient(conf)
	for {
		_, err := client.Ping(ctx, &core_grpc.RequestPing{})
		if err == nil {
			return
		}
	}
}

func randPort() int {
	port, err := tmnet.GetFreePort()
	if err != nil {
		panic(err)
	}
	return port
}

func makeAddrs() (string, string, string) {
	return fmt.Sprintf("tcp://127.0.0.1:%d", randPort()),
		fmt.Sprintf("tcp://127.0.0.1:%d", randPort()),
		fmt.Sprintf("tcp://127.0.0.1:%d", randPort())
}

func CreateConfig(testName string) *cfg.Config {
	c := cfg.ResetTestRoot(testName)

	// and we use random ports to run in parallel
	tm, rpc, grpc := makeAddrs()
	c.P2P.ListenAddress = tm
	c.RPC.ListenAddress = rpc
	c.RPC.CORSAllowedOrigins = []string{"https://tendermint.com/"}
	c.RPC.GRPCListenAddress = grpc
	return c
}

func GetGRPCClient(conf *cfg.Config) core_grpc.BroadcastAPIClient {
	grpcAddr := conf.RPC.GRPCListenAddress
	return core_grpc.StartGRPCClient(grpcAddr)
}

type ServiceCloser func(context.Context) error

func StartTendermint(ctx context.Context,
	conf *cfg.Config,
	app abci.Application,
	opts ...func(*Options)) (service.Service, ServiceCloser, error) {

	nodeOpts := &Options{}
	for _, opt := range opts {
		opt(nodeOpts)
	}
	var logger log.Logger
	if nodeOpts.suppressStdout {
		logger = log.NewNopLogger()
	} else {
		logger = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	}
	papp := proxy.NewLocalClientCreator(app)
	node, err := nm.New(conf, logger, papp, nil)
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}

	err = node.Start()
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}

	// wait for rpc
	waitForRPC(ctx, conf)
	waitForGRPC(ctx, conf)

	if !nodeOpts.suppressStdout {
		fmt.Println("Tendermint running!")
	}

	return node, func(ctx context.Context) error {
		if err := node.Stop(); err != nil {
			logger.Error("Error when trying to stop node", "err", err)
		}
		node.Wait()
		os.RemoveAll(conf.RootDir)
		return nil
	}, nil
}

// SuppressStdout is an option that tries to make sure the RPC test Tendermint
// node doesn't log anything to stdout.
func SuppressStdout(o *Options) {
	o.suppressStdout = true
}
