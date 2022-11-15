package orderbook

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

func NewMsgBid(pair *Pair, maxPrice, maxQuantity float64, ownerId uint64) *MsgBid {
	return &MsgBid{
		Pair: pair,
		BidOrder: &OrderBid{
			MaxPrice:    maxPrice,
			MaxQuantity: maxQuantity,
			OwnerId:     ownerId,
		},
	}
}

func (msg *MsgBid) Sign(pk crypto.PrivKey) error {
	sig, err := pk.Sign(msg.BidOrder.DeterministicSignatureBytes(msg.Pair))
	if err != nil {
		return err
	}
	msg.BidOrder.Signature = sig
	return nil
}

func (msg *MsgBid) ValidateBasic() error {
	if err := msg.BidOrder.ValidateBasic(); err != nil {
		return err
	}

	if err := msg.Pair.ValidateBasic(); err != nil {
		return err
	}

	if len(msg.BidOrder.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}

	return nil
}

func NewMsgAsk(pair *Pair, askPrice, quantity float64, ownerId uint64) *MsgAsk {
	return &MsgAsk{
		Pair: pair,
		AskOrder: &OrderAsk{
			AskPrice: askPrice,
			Quantity: quantity,
			OwnerId:  ownerId,
		},
	}
}

func (msg *MsgAsk) Sign(pk crypto.PrivKey) error {
	sig, err := pk.Sign(msg.AskOrder.DeterministicSignatureBytes(msg.Pair))
	if err != nil {
		return err
	}
	msg.AskOrder.Signature = sig
	return nil
}

func (msg *MsgAsk) ValidateBasic() error {
	if err := msg.AskOrder.ValidateBasic(); err != nil {
		return err
	}

	if err := msg.Pair.ValidateBasic(); err != nil {
		return err
	}

	return nil
}

func NewMsgCreateAccount(commodities ...*Commodity) (*MsgCreateAccount, crypto.PrivKey) {
	pk := ed25519.GenPrivKey()
	return &MsgCreateAccount{
		PublicKey:   pk.PubKey().Bytes(),
		Commodities: commodities,
	}, pk
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

func NewMsgRegisterPair(pair *Pair) *MsgRegisterPair {
	return &MsgRegisterPair{Pair: pair}
}

func (msg *MsgRegisterPair) ValidateBasic() error {
	return msg.Pair.ValidateBasic()
}

func NewCommodity(denom string, quantity float64) *Commodity {
	return &Commodity{
		Denom: denom,
		Quantity: quantity,
	}
}

func (c *Commodity) ValidateBasic() error {
	if c.Quantity <= 0 {
		return errors.New("quantity must be greater than zero")
	}

	return nil
}

func (p *Pair) ValidateBasic() error {
	if p.BuyersDenomination == "" || p.SellersDenomination == "" {
		return errors.New("inbound and outbound commodities must be present")
	}

	if p.BuyersDenomination == p.SellersDenomination {
		return errors.New("commodities must not be the same")
	}

	return nil
}

func (o *OrderBid) ValidateBasic() error {
	if o.MaxQuantity == 0 {
		return errors.New("max quantity must be non zero")
	}

	if o.MaxPrice <= 0 {
		return errors.New("min price must be greater than 0")
	}

	if len(o.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}

	return nil
}

func (o *OrderBid) ValidateSignature(pk crypto.PubKey, pair *Pair) bool {
	return pk.VerifySignature(o.DeterministicSignatureBytes(pair), o.Signature)
}

func (o *OrderBid) DeterministicSignatureBytes(pair *Pair) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(pair.SellersDenomination)
	buf.WriteString(pair.BuyersDenomination)
	bz := buf.Bytes()
	binary.BigEndian.PutUint64(bz, math.Float64bits(o.MaxQuantity))
	binary.BigEndian.PutUint64(bz, math.Float64bits(o.MaxPrice))
	return bz
}

func (m *MatchedOrder) ValidateBasic() error {
	if err := m.OrderAsk.ValidateBasic(); err != nil {
		return err
	}

	if err := m.OrderBid.ValidateBasic(); err != nil {
		return err
	}

	return nil
}

func (t *TradeSet) ValidateBasic() error {
	for _, matchedOrder := range t.MatchedOrders {
		if err := matchedOrder.ValidateBasic(); err != nil {
			return err
		}
		// checking if there is an account
		if matchedOrder.OrderAsk.OwnerId == 0 {
			return errors.New("must have an owner id more than zero")
		}
	}
	// validate the pairs are valid
	if err := t.Pair.ValidateBasic(); err != nil {
		return err
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

	if len(o.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}

	return nil
}

func (o *OrderAsk) ValidateSignature(pk crypto.PubKey, pair *Pair) bool {
	return pk.VerifySignature(o.DeterministicSignatureBytes(pair), o.Signature)
}

func (o *OrderAsk) DeterministicSignatureBytes(pair *Pair) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(pair.BuyersDenomination)
	buf.WriteString(pair.SellersDenomination)
	bz := buf.Bytes()
	binary.BigEndian.PutUint64(bz, math.Float64bits(o.Quantity))
	binary.BigEndian.PutUint64(bz, math.Float64bits(o.AskPrice))
	return bz
}

func (a *Account) FindCommidity(denom string) *Commodity {
	for _, c := range a.Commodities {
		if c.Denom == denom {
			return c
		}
	}

	return nil

}

func (a *Account) AddCommodity(c *Commodity) {
	curr := a.FindCommidity(c.Denom)
	if curr == nil {
		a.Commodities = append(a.Commodities, c)
	} else {
		curr.Quantity += c.Quantity
	}
}

func (a *Account) SubtractCommodity(c *Commodity) {
	curr := a.FindCommidity(c.Denom)
	if curr == nil {
		panic("trying to remove a commodity the account does not have")
	}
	curr.Quantity -= c.Quantity
}

func (msg *Msg) ValidateBasic() error { 
	switch m := msg.Sum.(type) {
	case *Msg_MsgRegisterPair:
		if err := m.MsgRegisterPair.ValidateBasic(); err != nil {
			return err
		}

	case *Msg_MsgCreateAccount:
		if err := m.MsgCreateAccount.ValidateBasic(); err != nil {
			return err
		}

	case *Msg_MsgBid:
		if err := m.MsgBid.ValidateBasic(); err != nil {
			return err
		}

	case *Msg_MsgAsk:
		if err := m.MsgAsk.ValidateBasic(); err != nil {
			return err
		}

	case *Msg_MsgTradeSet:
		if err := m.MsgTradeSet.TradeSet.ValidateBasic(); err != nil {
			return err
		}
	
	default:
		return errors.New("unknown tx")
	}

	return nil
}