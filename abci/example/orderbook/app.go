package orderbook

import (
	"fmt"

	"github.com/cosmos/gogoproto/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
)

var _ types.Application = (*StateMachine)(nil)

const Version = 1

//TO DO: Error codes

type StateMachine struct {
	// persisted state
	db dbm.DB

	// in-memory state
	accounts    map[uint64]*Account

	// ephemeral state (not used for the app hash) but for
	// convienience
	pairs     map[string]struct{} // lookup pairs
	commodity map[string]struct{} // lookup commodities

	// app-side mempool (also emphemeral)
	markets map[string]*Market // i.e. ATOM/USDC
}

func New() *StateMachine {
	return &StateMachine{}
}

func (sm *StateMachine) Info(req types.RequestInfo) types.ResponseInfo {
	return types.ResponseInfo{}
}

<<<<<<< HEAD
||||||| parent of b74494053 (process proposal checks)
func (sm *StateMachine) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	
	// execute the trade i.e. update everyone's accounts

	return types.ResponseDeliverTx{Code: 0}
}

=======
func (sm *StateMachine) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {

	// execute the trade i.e. update everyone's accounts

	return types.ResponseDeliverTx{Code: 0}
}

>>>>>>> b74494053 (process proposal checks)
// CheckTx to be stateless
func (sm *StateMachine) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	var msg = new(Msg)

	err := proto.Unmarshal(req.Tx, msg)
	if err != nil {
		return types.ResponseCheckTx{Code: 1} // decoding error
	}

	// validations for each msg below
	switch m := msg.Sum.(type) {
	case *Msg_MsgRegisterPair:
		// mustnt already have the same pair (also the reverse pairing)
		// inbound can also be outbound for pair

	case *Msg_MsgCreateAccount:

		if m.MsgCreateAccount.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: 3}
		}
		//check there is no other account with the same public key

	case *Msg_MsgBid:

		if err := m.MsgBid.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: 3, Log: err.Error()}
		}

		// check if account exists
		if _, ok := sm.accounts[m.MsgBid.BidOrder.OwnerId]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check the pair exists
		if _, ok := sm.pairs[m.MsgBid.Pair.String()]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check that the account has enough funds to support the bid (quantity * max_price)

	case *Msg_MsgAsk:

		if err := m.MsgAsk.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: 3, Log: err.Error()}
		}

		// check if account exists
		account, ok := sm.accounts[m.MsgAsk.AskOrder.OwnerId]
		if !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check the pair exists
		if _, ok := sm.pairs[m.MsgAsk.Pair.String()]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check if pair is registered
		if _, ok := sm.markets[m.MsgAsk.Pair.String()]; !ok {
			return types.ResponseCheckTx{Code: 4}

		}

		// check the account has a  enough quantity
		found := false
		for _, commodity := range account.Commodities {
			if commodity.Denom == m.MsgAsk.Pair.SellersDenomination {
				if m.MsgAsk.AskOrder.Quantity > commodity.Quantity {
					return types.ResponseCheckTx{Code: 4}
				}
				found = true
			}
		}
		if !found {
			return types.ResponseCheckTx{Code: 4}
		}

	default:
		return types.ResponseCheckTx{Code: 2} // unknown message type
	}

	return types.ResponseCheckTx{Code: 0}
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
		panic(fmt.Sprintf("unmarshalling tx: %w", err))
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

	// fetch and match all the bids and asks for each market
	


	// for _, market := range sm.markets {
	// 	tradeSet, err := market.Match()

	// 	for _, matchedOrder := range tradeSet.MatchedOrders {

	// 		// validate the trade:
	// 		// does the buyer and seller have sufficient funds

	// 		// add it to the set of txs

	// 	}

	// 	txs = append(txs, trades)
	// }

	// loop through the transactions provided by tendermint and look out for register pair and create account.
	// those should still be added.

	return types.ResponsePrepareProposal{Txs: req.Txs}
}

// Validating everything
func (sm *StateMachine) ProcessProposal(req types.RequestProcessProposal) types.ResponseProcessProposal {
	var msg = new(Msg)

	err := proto.Unmarshal(req.Txs[], msg)
	if err != nil {
		return types.ResponseCheckTx{Code: 1} // decoding error
	}

	// check if there is more than one account
	if ok := len(sm.accounts) >= 1; !ok {
		return types.ResponseProcessProposal{Code: 4}
	}

	// check if there is more than one commodity
	if ok := len(sm.commodities) >= 1; !ok {
		return types.ResponseProcessProposal{Code: 4}
	}

	// check if there is more than one pair
	if ok := len(sm.pairs) >= 1; !ok {
		return types.ResponseProcessProposal{Code: 4}
	}

	// check if there is a market
	if ok := len(sm.markets) >= 1; !ok {
		return types.ResponseProcessProposal{Code: 4}
	}

	return types.ResponseProcessProposal{
		Status: types.ResponseProcessProposal_ACCEPT}
}
