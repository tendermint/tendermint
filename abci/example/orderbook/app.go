package orderbook

import (
	"errors"
	"fmt"

	"github.com/cosmos/gogoproto/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

var _ types.Application = (*StateMachine)(nil)

//TO DO: Error codes

type StateMachine struct {

	// persisted state
	db dbm.DB

	// in-memory state
	accounts    map[uint64]*Account
	commodities map[string]*Commodity
	// app-side mempool
	markets map[string]*Market // i.e. ATOM/USDC
}

func New() *StateMachine {
	return &StateMachine{}
}

func (sm *StateMachine) Info(req types.RequestInfo) types.ResponseInfo {
	return types.ResponseInfo{}
}

func (sm *StateMachine) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	return types.ResponseDeliverTx{Code: 0}
}

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
		//mustnt already have the same pair
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
		if _, ok := sm.accounts[m.MsgBid.Order.Owner.Index]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check the commodity exists
		if _, ok := sm.commodities[m.MsgBid.Pair.OutboundCommodityDenom]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check if pair is registered
		if _, ok := sm.markets[m.MsgBid.Pair.String()]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

	case *Msg_MsgAsk:

		if err := m.MsgAsk.ValidateBasic(); err != nil {
			return types.ResponseCheckTx{Code: 3, Log: err.Error()}
		}

		// check if account exists
		if _, ok := sm.accounts[m.MsgAsk.Order.Owner.Index]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check the commodity exists
		if _, ok := sm.commodities[m.MsgAsk.Pair.OutboundCommodityDenom]; !ok {
			return types.ResponseCheckTx{Code: 4}
		}

		// check if pair is registered
		if _, ok := sm.markets[m.MsgAsk.Pair.String()]; !ok {
			return types.ResponseCheckTx{Code: 4}

		}

		// check the commodity has a high enough quantity
		if err := m.MsgAsk.Quantity >= sm.commodities[m.MsgAsk.Pair.OutboundCommodityDenom].Quantity; !ok {
			return types.ResponseCheckTx{Code: 4, Log: err.Error()}
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
	txs := make([][]byte, 0, len(req.Txs))
	var totalBytes int64
	for _, tx := range req.Txs {
		totalBytes += int64(len(tx))
		if totalBytes > req.MaxTxBytes {
			break
		}
		txs = append(txs, tx)
	}
	return types.ResponsePrepareProposal{Txs: txs}
}

func (sm *StateMachine) ProcessProposal(req types.RequestProcessProposal) types.ResponseProcessProposal {
	return types.ResponseProcessProposal{
		Status: types.ResponseProcessProposal_ACCEPT}
}

func (msg *MsgBid) ValidateBasic() error {
	if err := msg.Order.ValidateBasic(); err != nil {
		return err
	}

	if err := msg.Pair.ValidateBasic(); err != nil {
		return err
	}

	if len(msg.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}

	//quantity to be more than 0
	// price to not be 0

	return nil
}

func (msg *MsgAsk) ValidateBasic() error {
	if err := msg.Order.ValidateBasic(); err != nil {
		return err
	}

	if err := msg.Pair.ValidateBasic(); err != nil {
		return err
	}

	if len(msg.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}


	//quantity to be more than 0
	// price to not be 0

	return nil
}

func (msg *MsgCreateAccount) ValidateBasic() error {
	if len(msg.PublicKey) != ed25519.PubKeySize {
		return errors.New("invalid pub key size")
	}

	uniqueMap := make(map[string]struct{}, len(msg.Commodities))
	for _, c := range msg.Commodities {
		if err := c.ValidateBasic(); err != nil {
			return err
		}

		if _, ok := uniqueMap[c.Denom]; ok {
			return fmt.Errorf("commodity %s declared twice", c.Denom)
		}
		uniqueMap[c.Denom] = struct{}{}
	}

	return nil
}

func (msg *MsgRegisterPair) ValidateBasic() error {
	return msg.Pair.ValidateBasic()
}

func (c *Commodity) ValidateBasic() error {
	if c.Quantity <= 0 {
		return errors.New("quantity must be greater than zero")
	}
}

func (p *Pair) ValidateBasic() error {
	if p.InboundCommodityDenom == "" || p.OutboundCommodityDenom == "" {
		return errors.New("inbound and outbound commodities must be present")
	}

	if p.InboundCommodityDenom == p.OutboundCommodityDenom {
		return errors.New("commodities must not be the same")
	}

	return nil
}

func (o *OrderBid) ValidateBasic() error {
	if o.Quantity == 0 {
		return errors.New("quantity outbound must be non zero")
	}

	if o.MaxPrice <= 0 {
		return errors.New("min price must be greater than 0")
	}

	return nil
}

func (o *OrderAsk) ValidateBasic() error {
	if o.Quantity == 0 {
		return errors.New("quantity outbound must be non zero")
	}

	if o.AskPrice <= 0 {
		return errors.New("min price must be greater than 0")
	}

	return nil
}
