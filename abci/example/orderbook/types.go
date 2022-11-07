package orderbook

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/ed25519"
)

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

	//quantity to be more than 0
	// price to not be 0

	return nil
}

func (msg *MsgAsk) ValidateBasic() error {
	if err := msg.AskOrder.ValidateBasic(); err != nil {
		return err
	}

	if err := msg.Pair.ValidateBasic(); err != nil {
		return err
	}

	if len(msg.AskOrder.Signature) != ed25519.SignatureSize {
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

	return nil
}
// check signatures are valid

func (m *MatchedOrder) ValidateBasic() error {
	if len(m.OrderAsk.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}

	if len(m.OrderBid.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
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

	return nil
}

func (a *Account) FindCommidity(denom string) *Commodity {
	for _, c := range a.Commodities {
		if c.Denom == denom {
			return c
		}
	}
	return nil
}
