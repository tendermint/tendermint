package orderbook

import (
	"encoding/binary"

	"github.com/cosmos/gogoproto/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

var _ types.Application = (*StateMachine)(nil)

const Version = 1

const (
	// In tendermint a zero code is okay and all non zero codes are errors
	StatusOK = iota
	StatusErrDecoding
	StatusErrUnknownMessage
	StatusErrValidateBasic
	StatusErrNoAccount
	StatusErrAccountExists
	StatusErrNoPair
	StatusErrPairExists
	StatusErrInvalidOrder
	StatusErrUnacceptableMessage
	StatusErrNoCommodity
)

var (
	accountKey = []byte("account")
	pairKey = []byte("pair")
)

type StateMachine struct {
	// persisted state which is a key value store containing:
	//   accountID -> account
	//   pairID -> pair
	db dbm.DB

	// ephemeral state (not used for the app hash) but for
	// convienience
	accounts    map[uint64]*Account
	pairs       map[string]struct{} // lookup pairs
	commodities map[string]struct{} // lookup commodities
	publicKeys  map[string]struct{} // lookup existence of an account
	// a list of transactions that have been modified by the most recent block
	// and will need to result in an update to the db
	touchedAccounts map[uint64]struct{} 
	// new pairs added in this block which will needed to be added to the
	// db on "Commit"
	newPairs []*Pair

	// app-side mempool (also emphemeral)
	// this takes ask and bid transactions from `CheckTx`
	// and matches them as a "MatchedOrder" which is
	// then proposed in a block
	// 
	// it's important to note that there is no garbage collection
	// here. Bids and asks, potentially even invalid, will
	// continue to stay here until matched
	markets map[string]*Market // i.e. ATOM/USDC
}

func New(db dbm.DB) *StateMachine {
	// execute a database call that fetches the data for accounts
	StateMachine := StateMachine{
		accounts:    make(map[uint64]*Account),
		pairs:       make(map[string]struct{}),
		commodities: make(map[string]struct{}),
		publicKeys:  make(map[string]struct{}),
		markets:     make(map[string]*Market),
		db:          db,
	}
	return &StateMachine
}

func (sm *StateMachine) Info(req types.RequestInfo) types.ResponseInfo {
	return types.ResponseInfo{}
}

// CheckTx indicates which transactions
func (sm *StateMachine) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	var msg = new(Msg)

	err := proto.Unmarshal(req.Tx, msg)
	if err != nil {
		return types.ResponseCheckTx{Code: StatusErrDecoding, Log: err.Error()} // decoding error
	}

	if err := msg.ValidateBasic(); err != nil {
		return types.ResponseCheckTx{Code: StatusErrValidateBasic, Log: err.Error()}
	}

	// add either bids or asks to the market which will match them in PrepareProposal
	switch m := msg.Sum.(type) {
	case *Msg_MsgAsk:
		market, ok := sm.markets[m.MsgAsk.Pair.String()]
		if !ok {
			return types.ResponseCheckTx{Code: StatusErrNoPair}
		}
		market.AddAsk(m.MsgAsk.AskOrder)
	case *Msg_MsgBid:
		market, ok := sm.markets[m.MsgBid.Pair.String()]
		if !ok {
			return types.ResponseCheckTx{Code: StatusErrNoPair}
		}
		market.AddBid(m.MsgBid.BidOrder)
	}

	return types.ResponseCheckTx{Code: StatusOK}
}

func (sm *StateMachine) ValidateTx(msg *Msg) uint32 {
	if err := msg.ValidateBasic(); err != nil {
		return StatusErrValidateBasic
	}

	switch m := msg.Sum.(type) {
	case *Msg_MsgRegisterPair:
		pair := m.MsgRegisterPair.Pair
		if _, ok := sm.pairs[pair.String()]; ok {
			return StatusErrPairExists
		}

		reversePair := &Pair{BuyersDenomination: pair.SellersDenomination, SellersDenomination: pair.BuyersDenomination}
		if _, ok := sm.pairs[reversePair.String()]; ok {
			return StatusErrPairExists
		}

	case *Msg_MsgAsk, *Msg_MsgBid: // MsgAsk and MsgBid are not allowed individually - they need to be matched as a TradeSet
		return StatusErrUnacceptableMessage //Todo add logic around msg ask and bid to allow

	case *Msg_MsgCreateAccount:
		// check for duplicate accounts in state machine
		if _, ok := sm.publicKeys[string(m.MsgCreateAccount.PublicKey)]; ok {
			return StatusErrAccountExists
		}

		// check that each of the commodities is present in at least one trading pair
		for _, commodity := range m.MsgCreateAccount.Commodities {
			if _, exists := sm.commodities[commodity.Denom]; !exists {
				return StatusErrNoCommodity
			} 
		}

	case *Msg_MsgTradeSet:
		// check the pair exists
		if _, ok := sm.pairs[m.MsgTradeSet.TradeSet.Pair.String()]; !ok {
			return StatusErrNoPair
		}

		for _, order := range m.MsgTradeSet.TradeSet.MatchedOrders {
			// validate matched order i.e. users have funds and signatures are valid
			if !sm.isMatchedOrderValid(order, m.MsgTradeSet.TradeSet.Pair) {
				return StatusErrInvalidOrder
			}
		}

	default:
		return StatusErrUnknownMessage
	}

	return StatusOK
}

func (sm *StateMachine) Commit() types.ResponseCommit {
	batch := sm.db.NewBatch()

	// write to accounts that were modified by the last block
	for accountID := range sm.touchedAccounts {
		value, err := proto.Marshal(sm.accounts[accountID])
		if err != nil {
			panic(err)
		}
		var key []byte
		copy(key, accountKey)
		binary.BigEndian.PutUint64(key, accountID)

		batch.Set(key, value)
	}

	// write the new pairs that were added by the last block
	pairID := len(sm.pairs) - len(sm.newPairs)
	for id, pair := range sm.newPairs {
		value, err := proto.Marshal(pair)
		if err != nil {
			panic(err)
		}
		var key []byte
		copy(key, pairKey)
		binary.BigEndian.PutUint64(key, uint64(pairID + id))
		batch.Set(key, value)
	}

	batch.WriteSync()

	return types.ResponseCommit{Data: sm.hash()}
}

func (sm *StateMachine) hash() []byte {
	return tmhash.Sum([]byte("hash"))
}

func (sm *StateMachine) Query(req types.RequestQuery) types.ResponseQuery {
	return types.ResponseQuery{Code: 0}
}

func (sm *StateMachine) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	return types.ResponseInitChain{}
}

func (sm *StateMachine) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	// reset the new pairs
	sm.newPairs = make([]*Pair, 0)
	return types.ResponseBeginBlock{}
}

func (sm *StateMachine) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	var msg = new(Msg)

	err := proto.Unmarshal(req.Tx, msg)
	if err != nil {
		return types.ResponseDeliverTx{Code: StatusErrDecoding, Log: err.Error()} // decoding error
	}

	if status := sm.ValidateTx(msg); status != StatusOK {
		return types.ResponseDeliverTx{Code: status}
	}

	switch m := msg.Sum.(type) {
	case *Msg_MsgRegisterPair:
		sm.markets[m.MsgRegisterPair.Pair.String()] = NewMarket(m.MsgRegisterPair.Pair)
		sm.pairs[m.MsgRegisterPair.Pair.String()] = struct{}{}
		sm.commodities[m.MsgRegisterPair.Pair.BuyersDenomination] = struct{}{}
		sm.commodities[m.MsgRegisterPair.Pair.SellersDenomination] = struct{}{}
		sm.newPairs = append(sm.newPairs, m.MsgRegisterPair.Pair)

	case *Msg_MsgCreateAccount:
		nextAccountID := uint64(len(sm.accounts))
		sm.accounts[nextAccountID] = &Account{
			Index: nextAccountID,
			PublicKey: m.MsgCreateAccount.PublicKey,
			Commodities: m.MsgCreateAccount.Commodities,
		}
		sm.touchedAccounts[nextAccountID] = struct{}{}
		sm.publicKeys[string(m.MsgCreateAccount.PublicKey)] = struct{}{}

	case *Msg_MsgTradeSet:
		pair := m.MsgTradeSet.TradeSet.Pair
		for _, order := range m.MsgTradeSet.TradeSet.MatchedOrders {
			buyer := sm.accounts[order.OrderBid.OwnerId]
			seller := sm.accounts[order.OrderAsk.OwnerId]

			// the buyer gets quantity of the asset that the seller was selling
			buyer.AddCommodity(NewCommodity(pair.SellersDenomination, order.OrderAsk.Quantity))
			// the buyer gives up quantity * ask price of the buyers denomination
			buyer.SubtractCommodity(NewCommodity(pair.BuyersDenomination, order.OrderAsk.Quantity * order.OrderAsk.AskPrice))

			// the seller gets quantity * ask price of the asset that the buyer was paying with
			seller.AddCommodity(NewCommodity(pair.BuyersDenomination, order.OrderAsk.Quantity * order.OrderAsk.AskPrice))
			// the seller gives up quantity of the commodity they were selling
			seller.SubtractCommodity(NewCommodity(pair.SellersDenomination, order.OrderAsk.Quantity))

			// mark that these account have been touched
			sm.touchedAccounts[order.OrderBid.OwnerId] = struct{}{}
			sm.touchedAccounts[order.OrderAsk.OwnerId] = struct{}{}
		}

	default:
		return types.ResponseDeliverTx{Code: StatusErrUnknownMessage}
	}

	return types.ResponseDeliverTx{Code: 0}
}

// EndBlock is used to update consensus params and the validator set. For the orderbook,
// we keep both the same for thw 
func (sm *StateMachine) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	return types.ResponseEndBlock{}
}

func (sm *StateMachine) ListSnapshots(req types.RequestListSnapshots) types.ResponseListSnapshots {
	return types.ResponseListSnapshots{}
}

func (sm *StateMachine) OfferSnapshot(req types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	return types.ResponseOfferSnapshot{}
}

func (sm *StateMachine) LoadSnapshotChunk(req types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	return types.ResponseLoadSnapshotChunk{}
}

func (sm *StateMachine) ApplySnapshotChunk(req types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	return types.ResponseApplySnapshotChunk{}
}

func (sm *StateMachine) PrepareProposal(req types.RequestPrepareProposal) types.ResponsePrepareProposal {
	// declare transaction with the size of 0
	txs := make([][]byte, 0)

	// go through the transactions passed up via Tendermint first
	for _, tx := range req.Txs {
		var msg = new(Msg)
		err := proto.Unmarshal(tx, msg)
		if err != nil {
			panic(err)
		}

		// skip over the bids and asks that are proposed. We already have them
		if _, ok := msg.Sum.(*Msg_MsgBid); ok {
			continue
		}
		if _, ok := msg.Sum.(*Msg_MsgAsk); ok {
			continue
		}

		// make sure we're proposing valid transactions
		if status := sm.ValidateTx(msg); status != StatusOK {
			continue
		}	

		if len(txs)+len(tx) > int(req.MaxTxBytes) {
			return types.ResponsePrepareProposal{Txs: txs}
		}
		txs = append(txs, tx)
	}

	// fetch and match all the bids and asks for each market and add these
	for _, market := range sm.markets {
		tradeSet := market.Match()
		// tradesets into bytes and bytes into a transaction
		if tradeSet == nil {
			continue
		}

		tradeSet = sm.validateTradeSetAgainstState(tradeSet)
		if tradeSet == nil || len(tradeSet.MatchedOrders) == 0 {
			continue
		}

		// wrap this as a message typ
		msgTradeSet := &MsgTradeSet{TradeSet: tradeSet}
		bz, err := proto.Marshal(msgTradeSet)
		if err != nil {
			panic(err)
		}

		// check to see that we don't over populate the block
		if len(txs)+len(bz) > int(req.MaxTxBytes) {
			return types.ResponsePrepareProposal{Txs: txs}
		}
		txs = append(txs, bz)
	}

	return types.ResponsePrepareProposal{Txs: req.Txs}
}

// Process Proposal either rejects or accepts transactions
func (sm *StateMachine) ProcessProposal(req types.RequestProcessProposal) types.ResponseProcessProposal {
	for _, tx := range req.Txs {
		var msg = new(Msg)
		err := proto.Unmarshal(tx, msg)
		if err != nil {
			return rejectProposal()
		}

		if status := sm.ValidateTx(msg); status != StatusOK {
			return rejectProposal()
		}
	}

	return acceptProposal()
}

func (sm *StateMachine) validateTradeSetAgainstState(tradeSet *TradeSet) *TradeSet {
	output := &TradeSet{Pair: tradeSet.Pair}

	for _, matchedOrder := range tradeSet.MatchedOrders {
		if !sm.isMatchedOrderValid(matchedOrder, tradeSet.Pair) {
			continue
		}

		// yayy! this matched order is still valid and can be executed
		output.MatchedOrders = append(output.MatchedOrders, matchedOrder)
	}

	return output
}

func (sm *StateMachine) isMatchedOrderValid(order *MatchedOrder, pair *Pair) bool {
	bidOwner, exists := sm.accounts[order.OrderBid.OwnerId]
	if !exists {
		return false
	}
	askOwner, exists := sm.accounts[order.OrderAsk.OwnerId]
	if !exists {
		return false
	}

	askCommodities := askOwner.FindCommidity(pair.SellersDenomination)
	if askCommodities == nil {
		return false
	}
	buyCommodities := bidOwner.FindCommidity(pair.BuyersDenomination)
	if buyCommodities == nil {
		return false
	}

	// Seller has enough of the commodity
	if askCommodities.Quantity-order.OrderAsk.Quantity < 0 {
		return false
	}

	// Buyer has enough of the buying commodity
	if buyCommodities.Quantity-(order.OrderAsk.AskPrice*order.OrderAsk.Quantity) < 0 {
		return false
	}

	if !order.OrderAsk.ValidateSignature(ed25519.PubKey(askOwner.PublicKey), pair) {
		return false
	}
	if !order.OrderBid.ValidateSignature(ed25519.PubKey(bidOwner.PublicKey), pair) {
		return false
	}

	return true
}

func rejectProposal() types.ResponseProcessProposal {
	return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_REJECT}
}

func acceptProposal() types.ResponseProcessProposal {
	return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_ACCEPT}
}
