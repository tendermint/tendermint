package state

import (
	"bytes"
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/abci/example/dummy"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

var (
	privKey      = crypto.GenPrivKeyEd25519FromSecret([]byte("handshake_test"))
	chainID      = "handshake_chain"
	nBlocks      = 5
	mempool      = MockMempool{}
	testPartSize = 65536
	nTxsPerBlock = 10
)

//---------------------------------------
// Test block execution

func TestExecBlock(t *testing.T) {
	// TODO
}

//---------------------------------------
// Test handshake/replay

// Sync from scratch
func TestHandshakeReplayAll(t *testing.T) {
	testHandshakeReplay(t, 0)
}

// Sync many, not from scratch
func TestHandshakeReplaySome(t *testing.T) {
	testHandshakeReplay(t, 1)
}

// Sync from lagging by one
func TestHandshakeReplayOne(t *testing.T) {
	testHandshakeReplay(t, nBlocks-1)
}

// Sync from caught up
func TestHandshakeReplayNone(t *testing.T) {
	testHandshakeReplay(t, nBlocks)
}

// Make some blocks. Start a fresh app and apply n blocks. Then restart the app and sync it up with the remaining blocks
func testHandshakeReplay(t *testing.T, n int) {
	config := tendermint_test.ResetConfig("proxy_test_")

	state, store := stateAndStore(config)
	clientCreator := proxy.NewLocalClientCreator(dummy.NewPersistentDummyApplication(path.Join(config.GetString("db_dir"), "1")))
	clientCreator2 := proxy.NewLocalClientCreator(dummy.NewPersistentDummyApplication(path.Join(config.GetString("db_dir"), "2")))
	proxyApp := proxy.NewAppConns(config, clientCreator, NewHandshaker(config, state, store))
	_, err := proxyApp.Start()
	require.Nil(t, err, "Error starting proxy app connections: %v", err)
	chain := makeBlockchain(t, proxyApp, state, store)
	store.chain = chain //
	latestAppHash := state.AppHash
	proxyApp.Stop()

	if n > 0 {
		// start a new app without handshake, play n blocks
		proxyApp = proxy.NewAppConns(config, clientCreator2, nil)
		_, err := proxyApp.Start()
		require.Nil(t, err, "Error starting proxy app connections: %v", err)

		state2, store2 := stateAndStore(config)
		for i := 0; i < n; i++ {
			block := chain[i]
			err := state2.ApplyBlock(nil, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), mempool, store2)
			assert.Nil(t, err)
			assert.Equal(t, i*nTxsPerBlock+nTxsPerBlock, store2.nSavedTxResults)
		}
		proxyApp.Stop()
	}

	// now start it with the handshake
	handshaker := NewHandshaker(config, state, store)
	proxyApp = proxy.NewAppConns(config, clientCreator2, handshaker)
	_, err = proxyApp.Start()
	require.Nil(t, err, "Error starting proxy app connections: %v", err)

	// get the latest app hash from the app
	res, err := proxyApp.Query().InfoSync()
	assert.Nil(t, err)

	// the app hash should be synced up
	if !bytes.Equal(latestAppHash, res.LastBlockAppHash) {
		t.Fatalf("Expected app hashes to match after handshake/replay. got %X, expected %X", res.LastBlockAppHash, latestAppHash)
	}

	assert.Equal(t, handshaker.nBlocks, nBlocks-n,
		"Expected handshake to sync %d blocks, got %d", nBlocks-n, handshaker.nBlocks)
}

//--------------------------
// utils for making blocks

// make some bogus txs
func txsFunc(blockNum int) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		txs = append(txs, types.Tx([]byte{byte(blockNum), byte(i)}))
	}
	return txs
}

// sign a commit vote
func signCommit(height, round int, hash []byte, header types.PartSetHeader) *types.Vote {
	vote := &types.Vote{
		ValidatorIndex:   0,
		ValidatorAddress: privKey.PubKey().Address(),
		Height:           height,
		Round:            round,
		Type:             types.VoteTypePrecommit,
		BlockID:          types.BlockID{hash, header},
	}

	sig := privKey.Sign(types.SignBytes(chainID, vote))
	vote.Signature = sig
	return vote
}

// make a blockchain with one validator
func makeBlockchain(t *testing.T, proxyApp proxy.AppConns, state *State, store BlockStore) (blockchain []*types.Block) {

	prevHash := state.LastBlockID.Hash
	lastCommit := new(types.Commit)
	prevParts := types.PartSetHeader{}
	valHash := state.Validators.Hash()
	prevBlockID := types.BlockID{prevHash, prevParts}

	for i := 1; i <= nBlocks; i++ {
		block, parts := types.MakeBlock(i, chainID, txsFunc(i), lastCommit,
			prevBlockID, valHash, state.AppHash, testPartSize)
		fmt.Println(i)
		fmt.Println(prevBlockID)
		fmt.Println(block.LastBlockID)
		err := state.ApplyBlock(nil, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), mempool, store)
		require.Nil(t, err)

		voteSet := types.NewVoteSet(chainID, i, 0, types.VoteTypePrecommit, state.Validators)
		vote := signCommit(i, 0, block.Hash(), parts.Header())
		_, err = voteSet.AddVote(vote)
		require.Nil(t, err)

		blockchain = append(blockchain, block)
		prevHash = block.Hash()
		prevParts = parts.Header()
		lastCommit = voteSet.MakeCommit()
		prevBlockID = types.BlockID{prevHash, prevParts}
	}
	return blockchain
}

// fresh state and mock store
func stateAndStore(config cfg.Config) (*State, *mockBlockStore) {
	stateDB := dbm.NewMemDB()
	return MakeGenesisState(stateDB, &types.GenesisDoc{
		ChainID: chainID,
		Validators: []types.GenesisValidator{
			types.GenesisValidator{privKey.PubKey(), 10000, "test"},
		},
		AppHash: nil,
	}), NewMockBlockStore(config, nil)
}

//----------------------------------
// mock block store

type mockBlockStore struct {
	config          cfg.Config
	chain           []*types.Block
	nSavedTxResults int
}

func NewMockBlockStore(config cfg.Config, chain []*types.Block) *mockBlockStore {
	return &mockBlockStore{config, chain, 0}
}

func (bs *mockBlockStore) Height() int                       { return len(bs.chain) }
func (bs *mockBlockStore) LoadBlock(height int) *types.Block { return bs.chain[height-1] }
func (bs *mockBlockStore) LoadBlockMeta(height int) *types.BlockMeta {
	block := bs.chain[height-1]
	return &types.BlockMeta{
		Hash:        block.Hash(),
		Header:      block.Header,
		PartsHeader: block.MakePartSet(bs.config.GetInt("block_part_size")).Header(),
	}
}
func (bs *mockBlockStore) SaveTxResult(hash []byte, txResult *types.TxResult) {
	bs.nSavedTxResults++
}
