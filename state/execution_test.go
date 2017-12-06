package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/abci/example/dummy"
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

	// make block
	block := makeBlock(1, state)

	err = state.ApplyBlock(types.NopEventBus{}, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), types.MockMempool{})

	require.Nil(t, err)

	// TODO check state and mempool
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
	block, _ := types.MakeBlock(height, chainID, makeTxs(height), new(types.Commit),
		prevBlockID, valHash, state.AppHash, testPartSize)
	return block
}
