package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/abci/example/dummy"
	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

var (
	privKey      = crypto.GenPrivKeyEd25519FromSecret([]byte("execution_test"))
	chainID      = "execution_chain"
	testPartSize = 65536
	nTxsPerBlock = 10
)

func TestApplyBlock(t *testing.T) {
	cc := proxy.NewLocalClientCreator(dummy.NewDummyApplication())
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state := state()
	state.SetLogger(log.TestingLogger())

	block := makeBlock(1, state)

	err = state.ApplyBlock(types.NopEventBus{}, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), types.MockMempool{})
	require.Nil(t, err)

	// TODO check state and mempool
}

// TestBeginBlockAbsentValidators ensures we send absent validators list.
func TestBeginBlockAbsentValidators(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state := state()
	state.SetLogger(log.TestingLogger())

	// there were 2 validators
	val1PrivKey := crypto.GenPrivKeyEd25519()
	val2PrivKey := crypto.GenPrivKeyEd25519()
	lastValidators := types.NewValidatorSet([]*types.Validator{
		types.NewValidator(val1PrivKey.PubKey(), 10),
		types.NewValidator(val2PrivKey.PubKey(), 5),
	})

	// but last commit contains only the first validator
	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{prevHash, prevParts}
	lastCommit := &types.Commit{BlockID: prevBlockID, Precommits: []*types.Vote{
		{ValidatorIndex: 0},
	}}

	valHash := state.Validators.Hash()
	block, _ := types.MakeBlock(2, chainID, makeTxs(2), lastCommit,
		prevBlockID, valHash, state.AppHash, testPartSize)

	_, err = ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), lastValidators)
	require.Nil(t, err)

	// -> app must receive an index of the absent validator
	assert.Equal(t, []int32{1}, app.AbsentValidators)
}

//----------------------------------------------------------------------------

// make some bogus txs
func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func state() *State {
	s, _ := MakeGenesisState(dbm.NewMemDB(), &types.GenesisDoc{
		ChainID: chainID,
		Validators: []types.GenesisValidator{
			{privKey.PubKey(), 10000, "test"},
		},
		AppHash: nil,
	})
	return s
}

func makeBlock(height int64, state *State) *types.Block {
	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	valHash := state.Validators.Hash()
	prevBlockID := types.BlockID{prevHash, prevParts}
	block, _ := types.MakeBlock(height, chainID,
		makeTxs(height), state.LastBlockTotalTx,
		new(types.Commit), prevBlockID, valHash,
		state.AppHash, testPartSize)
	return block
}

//----------------------------------------------------------------------------

var _ abci.Application = (*testApp)(nil)

type testApp struct {
	abci.BaseApplication

	AbsentValidators []int32
}

func NewDummyApplication() *testApp {
	return &testApp{}
}

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	app.AbsentValidators = req.AbsentValidators
	return abci.ResponseBeginBlock{}
}

func (app *testApp) DeliverTx(tx []byte) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{Tags: []*abci.KVPair{}}
}

func (app *testApp) CheckTx(tx []byte) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{}
}

func (app *testApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{}
}

func (app *testApp) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	return
}
