package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/blocksync"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/proxy"
	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/indexer"
	"github.com/tendermint/tendermint/internal/statesync"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/strings"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/types"
)

const (
	// see README
	defaultPerPage = 30
	maxPerPage     = 100

	// SubscribeTimeout is the maximum time we wait to subscribe for an event.
	// must be less than the server's write timeout (see rpcserver.DefaultConfig)
	SubscribeTimeout = 5 * time.Second

	// genesisChunkSize is the maximum size, in bytes, of each
	// chunk in the genesis structure for the chunked API
	genesisChunkSize = 16 * 1024 * 1024 // 16
)

//----------------------------------------------
// These interfaces are used by RPC and must be thread safe

type consensusState interface {
	GetState() sm.State
	GetValidators() (int64, []*types.Validator)
	GetLastHeight() int64
	GetRoundStateJSON() ([]byte, error)
	GetRoundStateSimpleJSON() ([]byte, error)
}

type transport interface {
	Listeners() []string
	IsListening() bool
	NodeInfo() types.NodeInfo
}

type peerManager interface {
	Peers() []types.NodeID
	Addresses(types.NodeID) []p2p.NodeAddress
}

//----------------------------------------------
// Environment contains objects and interfaces used by the RPC. It is expected
// to be setup once during startup.
type Environment struct {
	// external, thread safe interfaces
	ProxyAppQuery   proxy.AppConnQuery
	ProxyAppMempool proxy.AppConnMempool

	// interfaces defined in types and above
	StateStore       sm.Store
	BlockStore       sm.BlockStore
	EvidencePool     sm.EvidencePool
	ConsensusState   consensusState
	ConsensusReactor *consensus.Reactor
	BlockSyncReactor *blocksync.Reactor

	// Legacy p2p stack
	P2PTransport transport

	// interfaces for new p2p interfaces
	PeerManager peerManager

	// objects
	PubKey            crypto.PubKey
	GenDoc            *types.GenesisDoc // cache the genesis structure
	EventSinks        []indexer.EventSink
	EventBus          *eventbus.EventBus // thread safe
	Mempool           mempool.Mempool
	StateSyncMetricer statesync.Metricer

	Logger log.Logger

	Config config.RPCConfig

	// cache of chunked genesis data.
	genChunks []string
}

//----------------------------------------------

func validatePage(pagePtr *int, perPage, totalCount int) (int, error) {
	// this can only happen if we haven't first run validatePerPage
	if perPage < 1 {
		panic(fmt.Errorf("%w (%d)", coretypes.ErrZeroOrNegativePerPage, perPage))
	}

	if pagePtr == nil { // no page parameter
		return 1, nil
	}

	pages := ((totalCount - 1) / perPage) + 1
	if pages == 0 {
		pages = 1 // one page (even if it's empty)
	}
	page := *pagePtr
	if page <= 0 || page > pages {
		return 1, fmt.Errorf("%w expected range: [1, %d], given %d", coretypes.ErrPageOutOfRange, pages, page)
	}

	return page, nil
}

func (env *Environment) validatePerPage(perPagePtr *int) int {
	if perPagePtr == nil { // no per_page parameter
		return defaultPerPage
	}

	perPage := *perPagePtr
	if perPage < 1 {
		return defaultPerPage
		// in unsafe mode there is no max on the page size but in safe mode
		// we cap it to maxPerPage
	} else if perPage > maxPerPage && !env.Config.Unsafe {
		return maxPerPage
	}
	return perPage
}

// InitGenesisChunks configures the environment and should be called on service
// startup.
func (env *Environment) InitGenesisChunks() error {
	if env.genChunks != nil {
		return nil
	}

	if env.GenDoc == nil {
		return nil
	}

	data, err := json.Marshal(env.GenDoc)
	if err != nil {
		return err
	}

	for i := 0; i < len(data); i += genesisChunkSize {
		end := i + genesisChunkSize

		if end > len(data) {
			end = len(data)
		}

		env.genChunks = append(env.genChunks, base64.StdEncoding.EncodeToString(data[i:end]))
	}

	return nil
}

func validateSkipCount(page, perPage int) int {
	skipCount := (page - 1) * perPage
	if skipCount < 0 {
		return 0
	}

	return skipCount
}

// latestHeight can be either latest committed or uncommitted (+1) height.
func (env *Environment) getHeight(latestHeight int64, heightPtr *int64) (int64, error) {
	if heightPtr != nil {
		height := *heightPtr
		if height <= 0 {
			return 0, fmt.Errorf("%w (requested height: %d)", coretypes.ErrZeroOrNegativeHeight, height)
		}
		if height > latestHeight {
			return 0, fmt.Errorf("%w (requested height: %d, blockchain height: %d)",
				coretypes.ErrHeightExceedsChainHead, height, latestHeight)
		}
		base := env.BlockStore.Base()
		if height < base {
			return 0, fmt.Errorf("%w (requested height: %d, base height: %d)", coretypes.ErrHeightNotAvailable, height, base)
		}
		return height, nil
	}
	return latestHeight, nil
}

func (env *Environment) latestUncommittedHeight() int64 {
	if env.ConsensusReactor != nil {
		// consensus reactor can be nil in inspect mode.

		nodeIsSyncing := env.ConsensusReactor.WaitSync()
		if nodeIsSyncing {
			return env.BlockStore.Height()
		}
	}
	return env.BlockStore.Height() + 1
}

// StartService constructs and starts listeners for the RPC service
// according to the config object, returning an error if the service
// cannot be constructed or started. The listeners, which provide
// access to the service, run until the context is canceled.
func (env *Environment) StartService(ctx context.Context, conf *config.Config) ([]net.Listener, error) {
	if err := env.InitGenesisChunks(); err != nil {
		return nil, err
	}

	listenAddrs := strings.SplitAndTrimEmpty(conf.RPC.ListenAddress, ",", " ")
	routes := NewRoutesMap(env, &RouteOptions{
		Unsafe: conf.RPC.Unsafe,
	})

	cfg := rpcserver.DefaultConfig()
	cfg.MaxBodyBytes = conf.RPC.MaxBodyBytes
	cfg.MaxHeaderBytes = conf.RPC.MaxHeaderBytes
	cfg.MaxOpenConnections = conf.RPC.MaxOpenConnections
	// If necessary adjust global WriteTimeout to ensure it's greater than
	// TimeoutBroadcastTxCommit.
	// See https://github.com/tendermint/tendermint/issues/3435
	if cfg.WriteTimeout <= conf.RPC.TimeoutBroadcastTxCommit {
		cfg.WriteTimeout = conf.RPC.TimeoutBroadcastTxCommit + 1*time.Second
	}

	// we may expose the rpc over both a unix and tcp socket
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		rpcLogger := env.Logger.With("module", "rpc-server")
		wmLogger := rpcLogger.With("protocol", "websocket")
		wm := rpcserver.NewWebsocketManager(wmLogger, routes,
			rpcserver.OnDisconnect(func(remoteAddr string) {
				err := env.EventBus.UnsubscribeAll(context.Background(), remoteAddr)
				if err != nil && err != tmpubsub.ErrSubscriptionNotFound {
					wmLogger.Error("Failed to unsubscribe addr from events", "addr", remoteAddr, "err", err)
				}
			}),
			rpcserver.ReadLimit(cfg.MaxBodyBytes),
		)
		mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(mux, routes, rpcLogger)
		listener, err := rpcserver.Listen(
			listenAddr,
			cfg.MaxOpenConnections,
		)
		if err != nil {
			return nil, err
		}

		var rootHandler http.Handler = mux
		if conf.RPC.IsCorsEnabled() {
			corsMiddleware := cors.New(cors.Options{
				AllowedOrigins: conf.RPC.CORSAllowedOrigins,
				AllowedMethods: conf.RPC.CORSAllowedMethods,
				AllowedHeaders: conf.RPC.CORSAllowedHeaders,
			})
			rootHandler = corsMiddleware.Handler(mux)
		}
		if conf.RPC.IsTLSEnabled() {
			go func() {
				if err := rpcserver.ServeTLS(
					ctx,
					listener,
					rootHandler,
					conf.RPC.CertFile(),
					conf.RPC.KeyFile(),
					rpcLogger,
					cfg,
				); err != nil {
					env.Logger.Error("error serving server with TLS", "err", err)
				}
			}()
		} else {
			go func() {
				if err := rpcserver.Serve(
					ctx,
					listener,
					rootHandler,
					rpcLogger,
					cfg,
				); err != nil {
					env.Logger.Error("error serving server", "err", err)
				}
			}()
		}

		listeners[i] = listener
	}

	return listeners, nil

}
