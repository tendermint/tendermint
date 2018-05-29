package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

var (
	privKey      = crypto.GenPrivKeyEd25519FromSecret([]byte("execution_test"))
	privKey2     = crypto.GenPrivKeyEd25519FromSecret([]byte("execution_test_2"))
	chainID      = "execution_chain"
	testPartSize = 65536
	nTxsPerBlock = 10
)

func TestApplyBlock(t *testing.T) {
	cc := proxy.NewLocalClientCreator(kvstore.NewKVStoreApplication())
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB := state(), dbm.NewMemDB()

	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(),
		types.MockMempool{}, types.MockEvidencePool{})

	block := makeBlock(state, 1)
	blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}

	state, err = blockExec.ApplyBlock(state, blockID, block)
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

	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{prevHash, prevParts}

	now := time.Now().UTC()
	testCases := []struct {
		desc                     string
		lastCommitPrecommits     []*types.Vote
		expectedAbsentValidators [][]byte
	}{
		{"none absent", []*types.Vote{{ValidatorIndex: 0, Timestamp: now, Type: types.VoteTypePrecommit}, {ValidatorIndex: 1, Timestamp: now}}, [][]byte{}},
		{"one absent", []*types.Vote{{ValidatorIndex: 0, Timestamp: now, Type: types.VoteTypePrecommit}, nil}, [][]byte{privKey2.PubKey().Bytes()}},
		{"multiple absent", []*types.Vote{nil, nil}, [][]byte{privKey.PubKey().Bytes(), privKey2.PubKey().Bytes()}},
	}

	for _, tc := range testCases {
		lastCommit := &types.Commit{BlockID: prevBlockID, Precommits: tc.lastCommitPrecommits}

		block, _ := state.MakeBlock(2, makeTxs(2), lastCommit)
		_, err = ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), state.Validators)
		require.Nil(t, err, tc.desc)

		// -> app must receive an index of the absent validator
		assert.Equal(t, tc.expectedAbsentValidators, app.AbsentValidators, tc.desc)
	}
}

// TestBeginBlockByzantineValidators ensures we send byzantine validators list.
func TestBeginBlockByzantineValidators(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state := state()

	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{prevHash, prevParts}

	height1, idx1, val1 := int64(8), 0, []byte("val1")
	height2, idx2, val2 := int64(3), 1, []byte("val2")
	ev1 := types.NewMockGoodEvidence(height1, idx1, val1)
	ev2 := types.NewMockGoodEvidence(height2, idx2, val2)

	testCases := []struct {
		desc                        string
		evidence                    []types.Evidence
		expectedByzantineValidators []abci.Evidence
	}{
		{"none byzantine", []types.Evidence{}, []abci.Evidence{}},
		{"one byzantine", []types.Evidence{ev1}, []abci.Evidence{{nil, ev1.Address(), ev1.Height(), int64(0)}}},
		{"multiple byzantine", []types.Evidence{ev1, ev2}, []abci.Evidence{
			{nil, ev1.Address(), ev1.Height(), int64(0)},
			{nil, ev2.Address(), ev2.Height(), int64(0)}}},
	}

	for _, tc := range testCases {
		lastCommit := &types.Commit{BlockID: prevBlockID}

		block, _ := state.MakeBlock(10, makeTxs(2), lastCommit)
		block.Evidence.Evidence = tc.evidence
		_, err = ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), state.Validators)
		require.Nil(t, err, tc.desc)

		// -> app must receive an index of the byzantine validator
		assert.Equal(t, tc.expectedByzantineValidators, app.ByzantineValidators, tc.desc)
	}
}

//----------------------------------------------------------------------------

// make some bogus txs
func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func state() State {
	s, _ := MakeGenesisState(&types.GenesisDoc{
		ChainID: chainID,
		Validators: []types.GenesisValidator{
			{privKey.PubKey(), 10000, "test"},
		},
		AppHash: nil,
	})
	return s
}

func makeBlock(state State, height int64) *types.Block {
	block, _ := state.MakeBlock(height, makeTxs(state.LastBlockHeight), new(types.Commit))
	return block
}

//----------------------------------------------------------------------------

var _ abci.Application = (*testApp)(nil)

type testApp struct {
	abci.BaseApplication

	AbsentValidators    [][]byte
	ByzantineValidators []abci.Evidence
}

func NewKVStoreApplication() *testApp {
	return &testApp{}
}

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	app.AbsentValidators = req.AbsentValidators
	app.ByzantineValidators = req.ByzantineValidators
	return abci.ResponseBeginBlock{}
}

func (app *testApp) DeliverTx(tx []byte) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{Tags: []cmn.KVPair{}}
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
