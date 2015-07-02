package state

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"sync"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/ed25519"
)

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
		// SANITY CHECK (binary decoding should catch bad vote types
		// before they get here (right?!)
		panic("Unknown vote type")
	}
}

type PrivValidator struct {
	Address    []byte                 `json:"address"`
	PubKey     account.PubKeyEd25519  `json:"pub_key"`
	PrivKey    account.PrivKeyEd25519 `json:"priv_key"`
	LastHeight int                    `json:"last_height"`
	LastRound  int                    `json:"last_round"`
	LastStep   int8                   `json:"last_step"`

	// For persistence.
	// Overloaded for testing.
	filePath string
	mtx      sync.Mutex
}

// Generates a new validator with private key.
func GenPrivValidator() *PrivValidator {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], CRandBytes(32))
	pubKeyBytes := ed25519.MakePublicKey(privKeyBytes)
	pubKey := account.PubKeyEd25519(pubKeyBytes[:])
	privKey := account.PrivKeyEd25519(privKeyBytes[:])
	return &PrivValidator{
		Address:    pubKey.Address(),
		PubKey:     pubKey,
		PrivKey:    privKey,
		LastHeight: 0,
		LastRound:  0,
		LastStep:   stepNone,
		filePath:   "",
	}
}

func LoadPrivValidator(filePath string) *PrivValidator {
	privValJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		Exit(err.Error())
	}
	privVal := binary.ReadJSON(&PrivValidator{}, privValJSONBytes, &err).(*PrivValidator)
	if err != nil {
		Exit(Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}
	privVal.filePath = filePath
	return privVal
}

func (privVal *PrivValidator) SetFile(filePath string) {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.filePath = filePath
}

func (privVal *PrivValidator) Save() {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	privVal.save()
}

func (privVal *PrivValidator) save() {
	if privVal.filePath == "" {
		// SANITY CHECK
		panic("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes := binary.JSONBytes(privVal)
	err := WriteFileAtomic(privVal.filePath, jsonBytes)
	if err != nil {
		// `@; BOOM!!!
		panic(err)
	}
}

func (privVal *PrivValidator) SignVote(chainID string, vote *types.Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()

	// If height regression, panic
	if privVal.LastHeight > vote.Height {
		return errors.New("Height regression in SignVote")
	}
	// More cases for when the height matches
	if privVal.LastHeight == vote.Height {
		// If round regression, panic
		if privVal.LastRound > vote.Round {
			return errors.New("Round regression in SignVote")
		}
		// If step regression, panic
		if privVal.LastRound == vote.Round && privVal.LastStep > voteToStep(vote) {
			return errors.New("Step regression in SignVote")
		}
	}

	// Persist height/round/step
	privVal.LastHeight = vote.Height
	privVal.LastRound = vote.Round
	privVal.LastStep = voteToStep(vote)
	privVal.save()

	// Sign
	privVal.SignVoteUnsafe(chainID, vote)
	return nil
}

func (privVal *PrivValidator) SignVoteUnsafe(chainID string, vote *types.Vote) {
	vote.Signature = privVal.PrivKey.Sign(account.SignBytes(chainID, vote)).(account.SignatureEd25519)
}

func (privVal *PrivValidator) SignProposal(chainID string, proposal *Proposal) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	if privVal.LastHeight < proposal.Height ||
		privVal.LastHeight == proposal.Height && privVal.LastRound < proposal.Round ||
		privVal.LastHeight == 0 && privVal.LastRound == 0 && privVal.LastStep == stepNone {

		// Persist height/round/step
		privVal.LastHeight = proposal.Height
		privVal.LastRound = proposal.Round
		privVal.LastStep = stepPropose
		privVal.save()

		// Sign
		proposal.Signature = privVal.PrivKey.Sign(account.SignBytes(chainID, proposal)).(account.SignatureEd25519)
		return nil
	} else {
		return errors.New(fmt.Sprintf("Attempt of duplicate signing of proposal: Height %v, Round %v", proposal.Height, proposal.Round))
	}
}

func (privVal *PrivValidator) SignRebondTx(chainID string, rebondTx *types.RebondTx) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()
	if privVal.LastHeight < rebondTx.Height {

		// Persist height/round/step
		// Prevent doing anything else for this rebondTx.Height.
		privVal.LastHeight = rebondTx.Height
		privVal.LastRound = math.MaxInt32 // MaxInt64 overflows on 32bit architectures.
		privVal.LastStep = math.MaxInt8
		privVal.save()

		// Sign
		rebondTx.Signature = privVal.PrivKey.Sign(account.SignBytes(chainID, rebondTx)).(account.SignatureEd25519)
		return nil
	} else {
		return errors.New(fmt.Sprintf("Attempt of duplicate signing of rebondTx: Height %v", rebondTx.Height))
	}
}

func (privVal *PrivValidator) String() string {
	return fmt.Sprintf("PrivValidator{%X LH:%v, LR:%v, LS:%v}", privVal.Address, privVal.LastHeight, privVal.LastRound, privVal.LastStep)
}
