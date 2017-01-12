package state

import (
	"bytes"
	"fmt"
	"math/rand"
	"path"
	"testing"

	// . "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-events"
	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"

	"github.com/tendermint/tmsp/example/counter"
	"github.com/tendermint/tmsp/example/dummy"
)

var (
	privKey      = crypto.GenPrivKeyEd25519FromSecret([]byte("handshake_test"))
	chainID      = "handshake_chain"
	nBlocks      = 5
	mempool      = MockMempool{}
	testPartSize = 65536
)

// Copied from consensus package - should eventually move this to a more central package
func subscribeToEvent(evsw types.EventSwitch, receiver, eventID string, chanCap int) chan interface{} {
	// listen for event
	ch := make(chan interface{}, chanCap)
	types.AddListenerForEvent(evsw, receiver, eventID, func(data types.TMEventData) {
		ch <- data
	})
	return ch
}

// Attempt to test ExecBlock func in execution.go
func TestExecBlock(t *testing.T) {
	config := tendermint_test.ResetConfig("proxy_test_") // test name?
	state, store := stateAndStore()

	// Using counter app so we can test valid and invalid transactions
	clientCreator := proxy.NewLocalClientCreator(counter.NewCounterApplication(true))
	proxyApp := proxy.NewAppConns(config, clientCreator, NewHandshaker(config, state, store))
	// Throw error if unable to start
	if _, err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}

	// kick off a blockchain
	prevHash := state.LastBlockID.Hash
	lastCommit := new(types.Commit)
	prevParts := types.PartSetHeader{}
	valHash := state.Validators.Hash()
	prevBlockID := types.BlockID{prevHash, prevParts}

	blockCount := 10 // arbitrary, was nBlocks+1
	n := 0
	// Count valid/invalid txns sent:
	valtxns := 0
	invtxns := 0
	// Count valid/invalid txns registered from events:
	valregs := 0
	invregs := 0
	for i := 1; i <= blockCount; i++ {
		var txs []types.Tx
		var txdata types.Tx

		// Create txs/txdata based on whether we want txn to be valid/invalid
		if valtxns == 0 || rand.Intn(100) < 70 { // 70% valid txns
			txdata = types.Tx([]byte{byte(0), byte(valtxns)})
			txs = append(txs, txdata)
			valtxns++
		} else {
			txdata = types.Tx([]byte{byte(0), byte(0)})
			txs = append(txs, txdata)
			invtxns++
		}

		// Create an event switch to listen for the newly inserted txn
		eventSwitch := events.NewEventSwitch()
		_, err := eventSwitch.Start()
		if err != nil {
			t.Fatalf("Failed to start switch: %v", err)
		}
		eventChannel := subscribeToEvent(eventSwitch, "tester", types.EventStringTx(txdata), 2500)

		// Create the block using the txs
		block, parts := types.MakeBlock(i, chainID, txs, lastCommit, prevBlockID, valHash, state.AppHash, testPartSize)
		fmt.Println("i =", i)
		fmt.Println("BlockID:", prevBlockID)

		err2 := state.ApplyBlock(eventSwitch, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), mempool)
		if err2 != nil {
			t.Fatal(i, err2)
		}

		voteSet := types.NewVoteSet(chainID, i, 0, types.VoteTypePrecommit, state.Validators)
		vote := signCommit(i, 0, block.Hash(), parts.Header())
		_, err = voteSet.AddVote(vote)
		if err != nil {
			t.Fatal(err)
		}

		prevHash = block.Hash()
		prevParts = parts.Header()
		lastCommit = voteSet.MakeCommit()
		prevBlockID = types.BlockID{prevHash, prevParts}

		select {
		case res := <-eventChannel:
			{
				fmt.Println("Got result: ", res.(types.EventDataTx).Log)
				if res.(types.EventDataTx).Code == tmsp.CodeType_OK {
					fmt.Println("Registered a valid transaction.")
					valregs++
				} else {
					fmt.Println("Registered an invalid transaction.")
					invregs++
				}
			}
		}

		n = i
	}

	// Potential errors
	if n < blockCount {
		t.Errorf("********************* saved %d blocks, expected %d **************", n, blockCount)
	} else {
		fmt.Println("********************* saved", n, "blocks out of", blockCount, "**************")
	}

	// Valid vs Invalid txns
	fmt.Println("Issued", valtxns, "valid transactions, ", invtxns, "invalid transactions.")
	fmt.Println("Registered", valregs, "valid transactions, ", invtxns, "invalid transactions.")
	if valtxns != valregs {
		t.Errorf("Registered valid transactions not equal to issued valid transactions.")
	}
	if invtxns != invregs {
		t.Errorf("Registered invalid transactions not equal to issued invalid transactions.")
	}

	proxyApp.Stop()
}

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

	state, store := stateAndStore()
	clientCreator := proxy.NewLocalClientCreator(dummy.NewPersistentDummyApplication(path.Join(config.GetString("db_dir"), "1")))
	clientCreator2 := proxy.NewLocalClientCreator(dummy.NewPersistentDummyApplication(path.Join(config.GetString("db_dir"), "2")))
	proxyApp := proxy.NewAppConns(config, clientCreator, NewHandshaker(config, state, store))
	if _, err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}
	chain := makeBlockchain(t, proxyApp, state)
	store.chain = chain //
	latestAppHash := state.AppHash
	proxyApp.Stop()

	if n > 0 {
		// start a new app without handshake, play n blocks
		proxyApp = proxy.NewAppConns(config, clientCreator2, nil)
		if _, err := proxyApp.Start(); err != nil {
			t.Fatalf("Error starting proxy app connections: %v", err)
		}
		state2, _ := stateAndStore()
		for i := 0; i < n; i++ {
			block := chain[i]
			err := state2.ApplyBlock(nil, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), mempool)
			if err != nil {
				t.Fatal(err)
			}
		}
		proxyApp.Stop()
	}

	// now start it with the handshake
	handshaker := NewHandshaker(config, state, store)
	proxyApp = proxy.NewAppConns(config, clientCreator2, handshaker)
	if _, err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}

	// get the latest app hash from the app
	r, _, blockInfo, _ := proxyApp.Query().InfoSync()
	if r.IsErr() {
		t.Fatal(r)
	}

	// the app hash should be synced up
	if !bytes.Equal(latestAppHash, blockInfo.AppHash) {
		t.Fatalf("Expected app hashes to match after handshake/replay. got %X, expected %X", blockInfo.AppHash, latestAppHash)
	}

	if handshaker.nBlocks != nBlocks-n {
		t.Fatalf("Expected handshake to sync %d blocks, got %d", nBlocks-n, handshaker.nBlocks)
	}

}

//--------------------------

// make some bogus txs
func txsFunc(blockNum int) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
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
func makeBlockchain(t *testing.T, proxyApp proxy.AppConns, state *State) (blockchain []*types.Block) {

	prevHash := state.LastBlockID.Hash
	lastCommit := new(types.Commit)
	prevParts := types.PartSetHeader{}
	valHash := state.Validators.Hash()
	prevBlockID := types.BlockID{prevHash, prevParts}

	for i := 1; i < nBlocks+1; i++ {
		block, parts := types.MakeBlock(i, chainID, txsFunc(i), lastCommit,
			prevBlockID, valHash, state.AppHash, testPartSize)
		fmt.Println(i)
		fmt.Println(prevBlockID)
		fmt.Println(block.LastBlockID)
		err := state.ApplyBlock(nil, proxyApp.Consensus(), block, block.MakePartSet(testPartSize).Header(), mempool)
		if err != nil {
			t.Fatal(i, err)
		}

		voteSet := types.NewVoteSet(chainID, i, 0, types.VoteTypePrecommit, state.Validators)
		vote := signCommit(i, 0, block.Hash(), parts.Header())
		_, err = voteSet.AddVote(vote)
		if err != nil {
			t.Fatal(err)
		}

		blockchain = append(blockchain, block)
		prevHash = block.Hash()
		prevParts = parts.Header()
		lastCommit = voteSet.MakeCommit()
		prevBlockID = types.BlockID{prevHash, prevParts}
	}
	return blockchain
}

// fresh state and mock store
func stateAndStore() (*State, *mockBlockStore) {
	stateDB := dbm.NewMemDB()
	return MakeGenesisState(stateDB, &types.GenesisDoc{
		ChainID: chainID,
		Validators: []types.GenesisValidator{
			types.GenesisValidator{privKey.PubKey(), 10000, "test"},
		},
		AppHash: nil,
	}), NewMockBlockStore(nil)
}

//----------------------------------
// mock block store

type mockBlockStore struct {
	chain []*types.Block
}

func NewMockBlockStore(chain []*types.Block) *mockBlockStore {
	return &mockBlockStore{chain}
}

func (bs *mockBlockStore) Height() int                       { return len(bs.chain) }
func (bs *mockBlockStore) LoadBlock(height int) *types.Block { return bs.chain[height-1] }
