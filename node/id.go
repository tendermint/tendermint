package node

import (
	"github.com/tendermint/go-crypto"
	"time"
)

type NodeID struct {
	Name   string
	PubKey crypto.PubKey
}

type PrivNodeID struct {
	NodeID
	PrivKey crypto.PrivKey
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
	Signature crypto.Signature
}

func (pnid *PrivNodeID) SignGreeting() *SignedNodeGreeting {
	//greeting := NodeGreeting{}
	return nil
}
