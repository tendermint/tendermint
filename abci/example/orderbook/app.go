package orderbook

import (
	"crypto/ed25519"
	"fmt"

	"github.com/cosmos/gogoproto/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
)

var _ types.Application = (*StateMachine)(nil)

const Version = 1

const (
	// In tendermint a zero code is okay and all non zero codes are errors
	StatusOK = iota
	ErrDecoding
	ErrUnknownMessage
	ErrValidateBasic
	ErrNoAccount
	ErrNoPair
)

type StateMachine struct {
	// persisted state
	db dbm.DB

	// ephemeral state (not used for the app hash) but for
	// convienience
	accounts    map[uint64]*Account
	pairs       map[string]struct{} // lookup pairs
	commodities map[string]struct{} // lookup commodities
	publicKeys  map[string]struct{} // lookup existence of an account

	// app-side mempool (also emphemeral)
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

// CheckTx to be stateless
func (sm *StateMachine) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	var msg = new(Msg)

	err := proto.Unmarshal(req.Tx, msg)
	if err != nil {
		return types.ResponseCheckTx{Code: ErrDecoding, Log: err.Error()} // decoding error
	}

	// validations for each msg below
	switch m := msg.Sum.(type) {
	case *Msg_MsgRegisterPair:
		if err := m.MsgRegisterPair.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: ErrValidateBasic, Log: err.Error()}
		}

	case *Msg_MsgCreateAccount:
		if err := m.MsgCreateAccount.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: ErrValidateBasic, Log: err.Error()}
		}

	case *Msg_MsgBid:

		if err := m.MsgBid.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: ErrValidateBasic, Log: err.Error()}
		}

		// check if account exists
		if _, ok := sm.accounts[m.MsgBid.BidOrder.OwnerId]; !ok {
			return types.ResponseCheckTx{Code: ErrNoAccount}
		}

		// check the pair exists
		if _, ok := sm.pairs[m.MsgBid.Pair.String()]; !ok {
			return types.ResponseCheckTx{Code: ErrNoPair}
		}

	case *Msg_MsgAsk:

		if err := m.MsgAsk.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: ErrValidateBasic, Log: err.Error()}
		}

		// check if account exists
		_, ok := sm.accounts[m.MsgAsk.AskOrder.OwnerId]
		if !ok {
			return types.ResponseCheckTx{Code: ErrNoAccount}
		}

		// check the pair exists
		if _, ok := sm.pairs[m.MsgAsk.Pair.String()]; !ok {
			return types.ResponseCheckTx{Code: ErrNoPair}
		}

	default:
		return types.ResponseCheckTx{Code: ErrUnknownMessage} // unknown message type
	}

	return types.ResponseCheckTx{Code: StatusOK}
}

func (sm *StateMachine) Commit() types.ResponseCommit {
	return types.ResponseCommit{}
}

func (sm *StateMachine) Query(req types.RequestQuery) types.ResponseQuery {
	return types.ResponseQuery{Code: 0}
}

func (sm *StateMachine) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	return types.ResponseInitChain{}
}

func (sm *StateMachine) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	return types.ResponseBeginBlock{}
}

func (sm *StateMachine) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	tradeSet := new(TradeSet)
	if err := proto.Unmarshal(req.Tx, tradeSet); err != nil {
		panic(fmt.Sprintf("unmarshalling tx: %v", err))
	}

	return types.ResponseDeliverTx{Code: 0}
}

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

	// fetch and match all the bids and asks for each market
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

	for _, tx := range req.Txs {
		var msg = new(Msg)
		err := proto.Unmarshal(tx, msg)
		if err != nil {
			panic(err)
		}

		switch m := msg.Sum.(type) {
		case *Msg_MsgRegisterPair:
			// run the validation checks to see if duplicates within the pairs
			pair := m.MsgRegisterPair.Pair
			if _, ok := sm.pairs[pair.String()]; ok {
				// this pair already exists so we skip over the message
				// garbage collection should pick it up
				continue
			}

			reversePair := &Pair{BuyersDenomination: pair.SellersDenomination, SellersDenomination: pair.BuyersDenomination}
			if _, ok := sm.pairs[reversePair.String()]; ok {
				// the reverse pair already exists so we skip over it
				continue
			}

			// check to see that we don't over populate the block
			if len(txs)+len(tx) > int(req.MaxTxBytes) {
				return types.ResponsePrepareProposal{Txs: txs}
			}
			txs = append(txs, tx)

		case *Msg_MsgCreateAccount:
			// check for duplicate accounts in sm
			if _, ok := sm.publicKeys[string(m.MsgCreateAccount.PublicKey)]; ok {
				continue
			}

			// check to see that we don't over populate the block
			if len(txs)+len(tx) > int(req.MaxTxBytes) {
				return types.ResponsePrepareProposal{Txs: txs}
			}
			txs = append(txs, tx)
		case *Msg_MsgAsk, *Msg_MsgBid:
			// Already have these in the market and are paring together so not necessary to include here
		default:
			panic(fmt.Sprintf("unknown msg type in prepare proposal %T", m))
		}
	}

	return types.ResponsePrepareProposal{Txs: req.Txs}
}

// Process Proposal either rejects or accepts transactions
func (sm *StateMachine) ProcessProposal(req types.RequestProcessProposal) types.ResponseProcessProposal {
	for _, tx := range req.Txs {
		var msg = new(Msg)
		err := proto.Unmarshal(tx, msg)
		if err != nil {
			panic(err)
		}

		switch m := msg.Sum.(type) {
		case *Msg_MsgRegisterPair:
			if err := m.MsgRegisterPair.ValidateBasic(); err != nil {
				return rejectProposal()
			}

			pair := m.MsgRegisterPair.Pair
			if _, ok := sm.pairs[pair.String()]; ok {
				return rejectProposal()
			}

			reversePair := &Pair{BuyersDenomination: pair.SellersDenomination, SellersDenomination: pair.BuyersDenomination}
			if _, ok := sm.pairs[reversePair.String()]; ok {
				return rejectProposal()
			}

			// TODO: verify the signature

		case *Msg_MsgAsk, *Msg_MsgBid: // MsgAsk and MsgBid are not allowed individually - they need to be matched as a TradeSet
			return rejectProposal()

		case *Msg_MsgCreateAccount:
			if err := m.MsgCreateAccount.ValidateBasic(); err != nil {
				return rejectProposal()
			}

			// check for duplicate accounts in sm
			if _, ok := sm.publicKeys[string(m.MsgCreateAccount.PublicKey)]; ok {
				return rejectProposal()
			}

			// TODO: verify the signature

		case *Msg_MsgTradeSet:
			// for each matched order
			// check the accounts exist, that the signatures are valid and that they have the available funds to make the swap
			if err := m.MsgTradeSet.TradeSet.ValidateBasic(); err != nil {
				return rejectProposal()
			}

			// check the pair exists
			if _, ok := sm.pairs[m.MsgTradeSet.TradeSet.Pair.String()]; !ok {
				return rejectProposal()
			}

			for _, order := range m.MsgTradeSet.TradeSet.MatchedOrders {
				if !sm.isMatchedOrderValid(order, m.MsgTradeSet.TradeSet.Pair) {
					return rejectProposal()
				}

				// bidOwner := sm.accounts[order.OrderBid.OwnerId]
				// askOwner := sm.accounts[order.OrderAsk.OwnerId]

				// ed25519.Verify(bidOwner.PublicKey, )
			}

			

		default:
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

	return true
}

func rejectProposal() types.ResponseProcessProposal {
	return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_REJECT}
}

func acceptProposal() types.ResponseProcessProposal {
	return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_ACCEPT}
}
