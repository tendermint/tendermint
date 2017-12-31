package types

import (
	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"

	"github.com/tendermint/tendermint/types"
)

// TODO: type ?
const (
	stepNone      = 0 // Used to distinguish the initial state
	stepPropose   = 1
	stepPrevote   = 2
	stepPrecommit = 3
)

func voteToStep(vote *types.Vote) int8 {
	switch vote.Type {
	case types.VoteTypePrevote:
		return stepPrevote
	case types.VoteTypePrecommit:
		return stepPrecommit
	default:
		panic("Unknown vote type")
	}
}

//----------------------------------------------------------------------------

type TypePrivValidator string

const (
	TypeUnencrypted = "unencrypted"
	TypeEncrypted   = "encrypted"
	TypeLedgerNanoS = "ledger-nano-s"
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
	SignVote(signer Signer, chainID string, vote *types.Vote) error
	SignProposal(signer Signer, chainID string, proposal *types.Proposal) error
	SignHeartbeat(signer Signer, chainID string, heartbeat *types.Heartbeat) error

	String() string // latest state
}

// PrivValidator is a Tendermint validator.
// It should use a CarefulSigner to ensure it never double signs.
type PrivValidator interface {
	Address() data.Bytes // redundant since .PubKey().Address()
	PubKey() crypto.PubKey

	SignVote(chainID string, vote *types.Vote) error
	SignProposal(chainID string, proposal *types.Proposal) error
	SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error
}

//----------------------------------------------------------------------------
