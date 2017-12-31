package types

import (
	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"

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
	SetSigner(Signer)

	GetCarefulSigner(TypePrivValidator) CarefulSigner
	SetCarefulSigner(CarefulSigner)
}

type DefaultStore struct {
	db            dbm.DB
	signer        Signer
	carefulSigner CarefulSigner
}

func NewDefaultStore(db dbm.DB) *DefaultStore {
	return &DefaultStore{
		db: db,
	}

}

func (pvs *DefaultStore) GetSigner(typ TypePrivValidator) Signer {
	return pvs.signer
}

func (pvs *DefaultStore) SetSigner(signer Signer) {
	pvs.signer = signer
}

func (pvs *DefaultStore) GetCarefulSigner(typ TypePrivValidator) CarefulSigner {
	return pvs.carefulSigner
}

func (pvs *DefaultStore) SetCarefulSigner(signer CarefulSigner) {
	pvs.carefulSigner = signer
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
