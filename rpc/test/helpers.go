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

	cfg "github.com/tendermint/tendermint/config"
	tmnet "github.com/tendermint/tendermint/libs/net"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
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

func createConfig() *cfg.Config {
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

// StartTendermint starts a test tendermint server in a go routine and returns when it is initialized
func StartTendermint(app abci.Application, opts ...func(*Options)) *nm.Node {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeOpts := defaultOptions
	for _, opt := range opts {
		opt(&nodeOpts)
	}
	node := NewTendermint(app, &nodeOpts)
	err := node.Start()
	if err != nil {
		panic(err)
	}

	cfg := node.Config()
	// wait for rpc
	waitForRPC(ctx, cfg)
	waitForGRPC(ctx, cfg)

	if !nodeOpts.suppressStdout {
		fmt.Println("Tendermint running!")
	}

	return node
}

// StopTendermint stops a test tendermint server, waits until it's stopped and
// cleans up test/config files.
func StopTendermint(node *nm.Node) {
	if err := node.Stop(); err != nil {
		node.Logger.Error("Error when tryint to stop node", "err", err)
	}
	node.Wait()
	os.RemoveAll(node.Config().RootDir)
}

// NewTendermint creates a new tendermint server and sleeps forever
func NewTendermint(app abci.Application, opts *Options) *nm.Node {
	// Create & start node
	config := createConfig()
	var logger log.Logger
	if opts.suppressStdout {
		logger = log.NewNopLogger()
	} else {
		logger = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	}
	pvKeyFile := config.PrivValidatorKeyFile()
	pvKeyStateFile := config.PrivValidatorStateFile()
	pv, err := privval.LoadOrGenFilePV(pvKeyFile, pvKeyStateFile)
	if err != nil {
		panic(err)
	}
	papp := proxy.NewLocalClientCreator(app)
	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		panic(err)
	}
	node, err := nm.NewNode(config, pv, nodeKey, papp,
		nm.DefaultGenesisDocProviderFunc(config),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(config.Instrumentation),
		logger)
	if err != nil {
		panic(err)
	}
	return node
}

// SuppressStdout is an option that tries to make sure the RPC test Tendermint
// node doesn't log anything to stdout.
func SuppressStdout(o *Options) {
	o.suppressStdout = true
}
