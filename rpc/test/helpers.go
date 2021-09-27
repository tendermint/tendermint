package rpctest

import (
	"context"
	"fmt"
	"os"
	"time"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/coretypes"
	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
	rpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

// Options helps with specifying some parameters for our RPC testing for greater
// control.
type Options struct {
	suppressStdout bool
}

func waitForRPC(ctx context.Context, conf *config.Config) {
	laddr := conf.RPC.ListenAddress
	client, err := rpcclient.New(laddr)
	if err != nil {
		panic(err)
	}
	result := new(coretypes.ResultStatus)
	for {
		_, err := client.Call(ctx, "status", map[string]interface{}{}, result)
		if err == nil {
			return
		}

		fmt.Println("error", err)
		time.Sleep(time.Millisecond)
	}
}

func waitForGRPC(ctx context.Context, conf *config.Config) {
	client := GetGRPCClient(conf)
	for {
		_, err := client.Ping(ctx, &coregrpc.RequestPing{})
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

func CreateConfig(testName string) *config.Config {
	c := config.ResetTestRoot(testName)

	// and we use random ports to run in parallel
	tm, rpc, grpc := makeAddrs()
	c.P2P.ListenAddress = tm
	c.RPC.ListenAddress = rpc
	c.RPC.CORSAllowedOrigins = []string{"https://tendermint.com/"}
	c.RPC.GRPCListenAddress = grpc
	return c
}

func GetGRPCClient(conf *config.Config) coregrpc.BroadcastAPIClient {
	grpcAddr := conf.RPC.GRPCListenAddress
	return coregrpc.StartGRPCClient(grpcAddr)
}

type ServiceCloser func(context.Context) error

func StartTendermint(ctx context.Context,
	conf *config.Config,
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
	papp := abciclient.NewLocalCreator(app)
	tmNode, err := node.New(conf, logger, papp, nil)
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}

	err = tmNode.Start()
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}

	// wait for rpc
	waitForRPC(ctx, conf)
	waitForGRPC(ctx, conf)

	if !nodeOpts.suppressStdout {
		fmt.Println("Tendermint running!")
	}

	return tmNode, func(ctx context.Context) error {
		if err := tmNode.Stop(); err != nil {
			logger.Error("Error when trying to stop node", "err", err)
		}
		tmNode.Wait()
		os.RemoveAll(conf.RootDir)
		return nil
	}, nil
}

// SuppressStdout is an option that tries to make sure the RPC test Tendermint
// node doesn't log anything to stdout.
func SuppressStdout(o *Options) {
	o.suppressStdout = true
}
