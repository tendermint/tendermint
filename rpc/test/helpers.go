package rpctest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"

	cfg "github.com/tendermint/tendermint/config"
	nm "github.com/tendermint/tendermint/internal/node"
	"github.com/tendermint/tendermint/internal/p2p"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	core_grpc "github.com/tendermint/tendermint/rpc/grpc"
	rpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

// Options helps with specifying some parameters for our RPC testing for greater
// control.
type Options struct {
	suppressStdout bool
	recreateConfig bool
}

var defaultOptions = Options{
	suppressStdout: false,
	recreateConfig: false,
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

// f**ing long, but unique for each test
func makePathname() string {
	// get path
	p, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	// fmt.Println(p)
	sep := string(filepath.Separator)
	return strings.ReplaceAll(p, sep, "_")
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

func CreateConfig() *cfg.Config {
	pathname := makePathname()
	c := cfg.ResetTestRoot(pathname)

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

func StartTendermint(ctx context.Context, conf *cfg.Config, app abci.Application, opts ...func(*Options)) (service.Service, ServiceCloser, error) {
	nodeOpts := defaultOptions
	for _, opt := range opts {
		opt(&nodeOpts)
	}

	var logger log.Logger
	if nodeOpts.suppressStdout {
		logger = log.NewNopLogger()
	} else {
		logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
		logger = log.NewFilter(logger, log.AllowError())
	}
	pvKeyFile := conf.PrivValidatorKeyFile()
	pvKeyStateFile := conf.PrivValidatorStateFile()
	pv, err := privval.LoadOrGenFilePV(pvKeyFile, pvKeyStateFile)
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}
	papp := proxy.NewLocalClientCreator(app)
	nodeKey, err := p2p.LoadOrGenNodeKey(conf.NodeKeyFile())
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}
	node, err := nm.NewNode(conf, pv, nodeKey, papp,
		nm.DefaultGenesisDocProviderFunc(conf),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(conf.Instrumentation),
		logger)
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
			logger.Error("Error when tryint to stop node", "err", err)
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
