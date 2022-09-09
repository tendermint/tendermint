package consensus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

type nodeGen struct {
	cfg      *config.Config
	app      abci.Application
	logger   log.Logger
	state    *sm.State
	storeDB  dbm.DB
	mempool  mempool.Mempool
	proxyApp abciclient.Client
	eventBus *eventbus.EventBus
}

func (g *nodeGen) initState(t *testing.T) {
	if g.state != nil {
		return
	}
	genDoc, err := types.GenesisDocFromFile(g.cfg.GenesisFile())
	require.NoError(t, err, "failed to read genesis file")
	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err, "failed to make genesis state")
	state.Version.Consensus.App = kvstore.ProtocolVersion
	g.state = &state
}

func (g *nodeGen) initApp(ctx context.Context, t *testing.T) {
	if g.app == nil {
		g.app = kvstore.NewApplication()
	}
	proxyLogger := g.logger.With("module", "proxy")
	proxyApp := proxy.New(abciclient.NewLocalClient(g.logger, g.app), proxyLogger, proxy.NopMetrics())
	g.proxyApp = proxyApp
	err := proxyApp.Start(ctx)
	require.NoError(t, err, "failed to start proxy app connections")
	t.Cleanup(proxyApp.Wait)
}

func (g *nodeGen) initStores() {
	if g.storeDB != nil {
		return
	}
	g.storeDB = dbm.NewMemDB()
}

func (g *nodeGen) initEventbus(ctx context.Context, t *testing.T) {
	g.eventBus = eventbus.NewDefault(g.logger.With("module", "events"))
	err := g.eventBus.Start(ctx)
	require.NoError(t, err, "failed to start event bus")
	t.Cleanup(func() {
		g.eventBus.Stop()
		g.eventBus.Wait()
	})
}

func (g *nodeGen) initMempool() {
	if g.mempool == nil {
		g.mempool = emptyMempool{}
	}
}

func (g *nodeGen) Generate(ctx context.Context, t *testing.T) *fakeNode {
	t.Helper()
	g.initStores()
	g.initApp(ctx, t)
	g.initState(t)
	g.initEventbus(ctx, t)
	g.initMempool()
	stateStore := sm.NewStore(g.storeDB)
	blockStore := store.NewBlockStore(g.storeDB)
	err := stateStore.Save(*g.state)
	require.NoError(t, err)

	evpool := sm.EmptyEvidencePool{}
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		g.proxyApp,
		g.mempool,
		evpool,
		blockStore,
		g.eventBus,
		sm.NopMetrics(),
	)
	blockExec.SetAppHashSize(g.cfg.Consensus.AppHashSize)
	csState, err := NewState(g.logger, g.cfg.Consensus, stateStore, blockExec, blockStore, g.mempool, evpool, g.eventBus)
	require.NoError(t, err)

	privValidator := privval.MustLoadOrGenFilePVFromConfig(g.cfg)
	if privValidator != nil {
		csState.SetPrivValidator(ctx, privValidator)
	}

	return &fakeNode{
		app:     g.app,
		csState: csState,
		pv:      privValidator,
	}
}

type fakeNode struct {
	app     abci.Application
	pv      types.PrivValidator
	csState *State
}

func newDefaultFakeNode(ctx context.Context, t *testing.T, logger log.Logger) *fakeNode {
	ng := nodeGen{cfg: getConfig(t), logger: logger}
	return ng.Generate(ctx, t)
}

func (n *fakeNode) start(ctx context.Context, t *testing.T) {
	require.NoError(t, n.csState.Start(ctx))
	t.Cleanup(n.csState.Wait)
}

func (n *fakeNode) stop() {
	n.csState.Stop()
}
