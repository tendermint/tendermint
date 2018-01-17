package types

import (
	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
)

//----------------------------------------------------------------------------

// ValidatorID contains the identity of the validator.
type ValidatorID struct {
	Address data.Bytes    `json:"address"`
	PubKey  crypto.PubKey `json:"pub_key"`
}

//----------------------------------------------------------------------------

// Signer signs a message.
type Signer interface {
	Sign(msg []byte) (crypto.Signature, error)
}

//----------------------------------------------------------------------------

// PrivValidator is a Tendermint validator.
// It should use a CarefulSigner to ensure it never double signs.
type PrivValidator interface {
	Address() data.Bytes // redundant since .PubKey().Address()
	PubKey() crypto.PubKey

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
	SignHeartbeat(chainID string, heartbeat *Heartbeat) error
}

//----------------------------------------------------------------------------
