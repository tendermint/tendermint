package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/abci/example/dummy"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/state/txindex"
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
	_, err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state := state()
	state.SetLogger(log.TestingLogger())
	indexer := &dummyIndexer{0}
	state.TxIndexer = indexer

	// make block
	block := makeBlock(1, state)

	err = state.ApplyBlock(nil, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), types.MockMempool{})

	require.Nil(t, err)
	assert.Equal(t, nTxsPerBlock, indexer.Indexed) // test indexing works

	// TODO check state and mempool
}

//----------------------------------------------------------------------------

// make some bogus txs
func makeTxs(blockNum int) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		txs = append(txs, types.Tx([]byte{byte(blockNum), byte(i)}))
	}
	return txs
}

func state() *State {
	return MakeGenesisState(dbm.NewMemDB(), &types.GenesisDoc{
		ChainID: chainID,
		Validators: []types.GenesisValidator{
			types.GenesisValidator{privKey.PubKey(), 10000, "test"},
		},
		AppHash: nil,
	})
}

func makeBlock(num int, state *State) *types.Block {
	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	valHash := state.Validators.Hash()
	prevBlockID := types.BlockID{prevHash, prevParts}
	block, _ := types.MakeBlock(num, chainID, makeTxs(num), new(types.Commit),
		prevBlockID, valHash, state.AppHash, testPartSize)
	return block
}

// dummyIndexer increments counter every time we index transaction.
type dummyIndexer struct {
	Indexed int
}

func (indexer *dummyIndexer) Get(hash []byte) (*types.TxResult, error) {
	return nil, nil
}
func (indexer *dummyIndexer) AddBatch(batch *txindex.Batch) error {
	indexer.Indexed += batch.Size()
	return nil
}
