package mempool

import (
	"fmt"
	"sync"
	"testing"
	"time"

	acm "github.com/tendermint/tendermint/account"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var someAddr = []byte("ABCDEFGHIJABCDEFGHIJ")

// number of txs
var nTxs = 100

// what the ResetInfo should look like after ResetForBlockAndState
var TestResetInfoData = ResetInfo{
	Included: []Range{
		Range{0, 5},
		Range{10, 10},
		Range{30, 5},
	},
	Invalid: []Range{
		Range{5, 5},
		Range{20, 8},  // let 28 and 29 be valid
		Range{35, 64}, // let 99 be valid
	},
}

// inverse of the ResetInfo
var notInvalidNotIncluded = map[int]struct{}{
	28: struct{}{},
	29: struct{}{},
	99: struct{}{},
}

func newSendTx(t *testing.T, mempool *Mempool, from *acm.PrivAccount, to []byte, amt int64) types.Tx {
	tx := types.NewSendTx()
	tx.AddInput(mempool.GetCache(), from.PubKey, amt)
	tx.AddOutput(to, amt)
	tx.SignInput(config.GetString("chain_id"), 0, from)
	if err := mempool.AddTx(tx); err != nil {
		t.Fatal(err)
	}
	return tx
}

func addTxs(t *testing.T, mempool *Mempool, lastAcc *acm.PrivAccount, privAccs []*acm.PrivAccount) []types.Tx {
	txs := make([]types.Tx, nTxs)
	for i := 0; i < nTxs; i++ {
		if _, ok := notInvalidNotIncluded[i]; ok {
			txs[i] = newSendTx(t, mempool, lastAcc, someAddr, 10)
		} else {
			txs[i] = newSendTx(t, mempool, privAccs[i%len(privAccs)], privAccs[(i+1)%len(privAccs)].Address, 5)
		}
	}
	return txs
}

func makeBlock(mempool *Mempool) *types.Block {
	txs := mempool.GetProposalTxs()
	var includedTxs []types.Tx
	for _, rid := range TestResetInfoData.Included {
		includedTxs = append(includedTxs, txs[rid.Start:rid.Start+rid.Length]...)
	}

	mempool.mtx.Lock()
	state := mempool.state
	state.LastBlockHeight += 1
	mempool.mtx.Unlock()
	return &types.Block{
		Header: &types.Header{
			ChainID: state.ChainID,
			Height:  state.LastBlockHeight,
			NumTxs:  len(includedTxs),
		},
		Data: &types.Data{
			Txs: includedTxs,
		},
	}
}

// Add txs. Grab chunks to put in block. All the others become invalid because of nonce errors except those in notInvalidNotIncluded
func TestResetInfo(t *testing.T) {
	amtPerAccount := int64(100000)
	state, privAccs, _ := sm.RandGenesisState(6, false, amtPerAccount, 1, true, 100)

	mempool := NewMempool(state)

	lastAcc := privAccs[5] // we save him (his tx wont become invalid)
	privAccs = privAccs[:5]

	txs := addTxs(t, mempool, lastAcc, privAccs)

	// its actually an invalid block since we're skipping nonces
	// but all we care about is how the mempool responds after
	block := makeBlock(mempool)

	mempool.ResetForBlockAndState(block, state)

	ri := mempool.resetInfo

	if len(ri.Included) != len(TestResetInfoData.Included) {
		t.Fatalf("invalid number of included ranges. Got %d, expected %d\n", len(ri.Included), len(TestResetInfoData.Included))
	}

	if len(ri.Invalid) != len(TestResetInfoData.Invalid) {
		t.Fatalf("invalid number of invalid ranges. Got %d, expected %d\n", len(ri.Invalid), len(TestResetInfoData.Invalid))
	}

	for i, rid := range ri.Included {
		inc := TestResetInfoData.Included[i]
		if rid.Start != inc.Start {
			t.Fatalf("Invalid start of range. Got %d, expected %d\n", inc.Start, rid.Start)
		}
		if rid.Length != inc.Length {
			t.Fatalf("Invalid length of range. Got %d, expected %d\n", inc.Length, rid.Length)
		}
	}

	txs = mempool.GetProposalTxs()
	if len(txs) != len(notInvalidNotIncluded) {
		t.Fatalf("Expected %d txs left in mempool. Got %d", len(notInvalidNotIncluded), len(txs))
	}
}

//------------------------------------------------------------------------------------------

type TestPeer struct {
	sync.Mutex
	running bool
	height  int

	t *testing.T

	received int
	txs      map[string]int

	timeoutFail int

	done chan int
}

func newPeer(t *testing.T, state *sm.State) *TestPeer {
	return &TestPeer{
		running: true,
		height:  state.LastBlockHeight,
		t:       t,
		txs:     make(map[string]int),
		done:    make(chan int),
	}
}

func (tp *TestPeer) IsRunning() bool {
	tp.Lock()
	defer tp.Unlock()
	return tp.running
}

func (tp *TestPeer) SetRunning(running bool) {
	tp.Lock()
	defer tp.Unlock()
	tp.running = running
}

func (tp *TestPeer) Send(chID byte, msg interface{}) bool {
	if tp.timeoutFail > 0 {
		time.Sleep(time.Second * time.Duration(tp.timeoutFail))
		return false
	}
	tx := msg.(*TxMessage).Tx
	id := types.TxID(config.GetString("chain_id"), tx)
	if _, ok := tp.txs[string(id)]; ok {
		tp.t.Fatal("received the same tx twice!")
	}
	tp.txs[string(id)] = tp.received
	tp.received += 1
	tp.done <- tp.received
	return true
}

func (tp *TestPeer) Get(key string) interface{} {
	return tp
}

func (tp *TestPeer) GetHeight() int {
	return tp.height
}

func TestBroadcast(t *testing.T) {
	state, privAccs, _ := sm.RandGenesisState(6, false, 10000, 1, true, 100)
	mempool := NewMempool(state)
	reactor := NewMempoolReactor(mempool)
	reactor.Start()

	lastAcc := privAccs[5] // we save him (his tx wont become invalid)
	privAccs = privAccs[:5]

	peer := newPeer(t, state)
	newBlockChan := make(chan ResetInfo)
	tickerChan := make(chan time.Time)
	go reactor.broadcastTxRoutine(tickerChan, newBlockChan, peer)

	// we don't broadcast any before updating
	fmt.Println("dont broadcast any")
	addTxs(t, mempool, lastAcc, privAccs)
	block := makeBlock(mempool)
	mempool.ResetForBlockAndState(block, state)
	newBlockChan <- mempool.resetInfo
	peer.height = mempool.resetInfo.Height
	tickerChan <- time.Now()
	pullTxs(t, peer, len(mempool.txs)) // should have sent whatever txs are left (3)

	toBroadcast := []int{1, 3, 7, 9, 11, 12, 18, 20, 21, 28, 29, 30, 31, 34, 35, 36, 50, 90, 99, 100}
	for _, N := range toBroadcast {
		peer = resetPeer(t, reactor, mempool, state, tickerChan, newBlockChan, peer)

		// we broadcast N txs before updating
		fmt.Println("broadcast", N)
		addTxs(t, mempool, lastAcc, privAccs)
		txsToSendPerCheck = N
		tickerChan <- time.Now()
		pullTxs(t, peer, txsToSendPerCheck) // should have sent N txs
		block = makeBlock(mempool)
		mempool.ResetForBlockAndState(block, state)
		newBlockChan <- mempool.resetInfo
		peer.height = mempool.resetInfo.Height
		txsToSendPerCheck = 100
		tickerChan <- time.Now()
		left := len(mempool.txs)
		if N > 99 {
			left -= 3
		} else if N > 29 {
			left -= 2
		} else if N > 28 {
			left -= 1
		}
		pullTxs(t, peer, left) // should have sent whatever txs are left that havent been sent
	}
}

func pullTxs(t *testing.T, peer *TestPeer, N int) {
	timer := time.NewTicker(time.Second * 2)
	for i := 0; i < N; i++ {
		select {
		case <-peer.done:
		case <-timer.C:
			panic(fmt.Sprintf("invalid number of received messages. Got %d, expected %d\n", i, N))
		}
	}

	if N == 0 {
		select {
		case <-peer.done:
			t.Fatalf("should not have sent any more txs")
		case <-timer.C:
		}
	}
}

func resetPeer(t *testing.T, reactor *MempoolReactor, mempool *Mempool, state *sm.State, tickerChan chan time.Time, newBlockChan chan ResetInfo, peer *TestPeer) *TestPeer {
	// reset peer
	mempool.txs = []types.Tx{}
	mempool.state = state
	mempool.cache = sm.NewBlockCache(state)
	peer.SetRunning(false)
	tickerChan <- time.Now()
	peer = newPeer(t, state)
	go reactor.broadcastTxRoutine(tickerChan, newBlockChan, peer)
	return peer
}
