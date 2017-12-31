package types

import (
	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-data"
	cmn "github.com/tendermint/tmlibs/common"
)

// TODO: type ?
const (
	stepNone      = 0 // Used to distinguish the initial state
	stepPropose   = 1
	stepPrevote   = 2
	stepPrecommit = 3
)

func voteToStep(vote *Vote) int8 {
	switch vote.Type {
	case VoteTypePrevote:
		return stepPrevote
	case VoteTypePrecommit:
		return stepPrecommit
	default:
		cmn.PanicSanity("Unknown vote type")
		return 0
	}
}

//----------------------------------------------------------------------------

type TypePrivValidator string

const (
	TypePrivValidatorKeyStoreUnencrypted = "keystore-unencrypted"
	TypePrivValidatorKeyStoreEncrypted   = "keystore-encrypted"
	TypePrivValidatorLedgerNanoS         = "ledger-nano-s"
	// ...
)

// this is saved in priv_validator.json.
type PrivValidatorInfo struct {
	ID   ValidatorID       `json:"id"`
	Type TypePrivValidator `json:"type"`
}

// ValidatorID contains the identity of the validator.
type ValidatorID struct {
	Address data.Bytes    `json:"address"`
	PubKey  crypto.PubKey `json:"pub_key"`
}

type PrivValidatorStore interface {
	GetSigner(TypePrivValidator) Signer
	GetCarefulSigner(TypePrivValidator) CarefulSigner
}

//----------------------------------------------------------------------------

// Signer is an interface that defines how to sign messages.
// It is the caller's duty to verify the msg before calling Sign,
// eg. to avoid double signing.
type Signer interface {
	Sign(msg []byte) (crypto.Signature, error)
}

// CarefulSigner signs votes, proposals, and heartbeats,
// but is careful not to double sign!
type CarefulSigner interface {
	SignVote(signer Signer, chainID string, vote *Vote) error
	SignProposal(signer Signer, chainID string, proposal *Proposal) error
	SignHeartbeat(signer Signer, chainID string, heartbeat *Heartbeat) error

	String() string // latest state
}

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
