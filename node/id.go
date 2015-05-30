package node

import (
	acm "github.com/tendermint/tendermint/account"
	"time"
)

type NodeID struct {
	Name   string
	PubKey acm.PubKey
}

type PrivNodeID struct {
	NodeID
	PrivKey acm.PrivKey
}

type NodeGreeting struct {
	NodeID
	Version string
	ChainID string
	Message string
	Time    time.Time
}

type SignedNodeGreeting struct {
	NodeGreeting
	Signature acm.Signature
}

func (pnid *PrivNodeID) SignGreeting() *SignedNodeGreeting {
	//greeting := NodeGreeting{}
	return nil
}
