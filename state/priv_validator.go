package state

// TODO: This logic is crude. Should be more transactional.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/config"
	. "github.com/tendermint/tendermint/consensus/types"

	"github.com/tendermint/go-ed25519"
)

const (
	stepNone      = 0 // Used to distinguish the initial state
	stepPropose   = 1
	stepPrevote   = 2
	stepPrecommit = 3
	stepCommit    = 4
)

func voteToStep(vote *Vote) uint8 {
	switch vote.Type {
	case VoteTypePrevote:
		return stepPrevote
	case VoteTypePrecommit:
		return stepPrecommit
	case VoteTypeCommit:
		return stepCommit
	default:
		panic("Unknown vote type")
	}
}

type PrivValidator struct {
	Address    []byte
	PubKey     PubKeyEd25519
	PrivKey    PrivKeyEd25519
	LastHeight uint
	LastRound  uint
	LastStep   uint8

	// For persistence.
	// Overloaded for testing.
	filename string
}

// Generates a new validator with private key.
func GenPrivValidator() *PrivValidator {
	privKeyBytes := CRandBytes(32)
	pubKeyBytes := ed25519.MakePubKey(privKeyBytes)
	pubKey := PubKeyEd25519{pubKeyBytes}
	privKey := PrivKeyEd25519{pubKeyBytes, privKeyBytes}
	return &PrivValidator{
		Address:    pubKey.Address(),
		PubKey:     pubKey,
		PrivKey:    privKey,
		LastHeight: 0,
		LastRound:  0,
		LastStep:   stepNone,
		filename:   PrivValidatorFile(),
	}
}

type PrivValidatorJSON struct {
	Address    string
	PubKey     string
	PrivKey    string
	LastHeight uint
	LastRound  uint
	LastStep   uint8
}

func LoadPrivValidator() *PrivValidator {
	privValJSONBytes, err := ioutil.ReadFile(PrivValidatorFile())
	if err != nil {
		panic(err)
	}
	privValJSON := PrivValidatorJSON{}
	err = json.Unmarshal(privValJSONBytes, &privValJSON)
	if err != nil {
		panic(err)
	}
	address, err := base64.StdEncoding.DecodeString(privValJSON.Address)
	if err != nil {
		panic(err)
	}
	pubKeyBytes, err := base64.StdEncoding.DecodeString(privValJSON.PubKey)
	if err != nil {
		panic(err)
	}
	privKeyBytes, err := base64.StdEncoding.DecodeString(privValJSON.PrivKey)
	if err != nil {
		panic(err)
	}
	n := new(int64)
	privVal := &PrivValidator{
		Address:    address,
		PubKey:     ReadBinary(PubKeyEd25519{}, bytes.NewReader(pubKeyBytes), n, &err).(PubKeyEd25519),
		PrivKey:    ReadBinary(PrivKeyEd25519{}, bytes.NewReader(privKeyBytes), n, &err).(PrivKeyEd25519),
		LastHeight: privValJSON.LastHeight,
		LastRound:  privValJSON.LastRound,
		LastStep:   privValJSON.LastStep,
	}
	if err != nil {
		panic(err)
	}
	return privVal
}

func (privVal *PrivValidator) Save() {
	privValJSON := PrivValidatorJSON{
		Address:    base64.StdEncoding.EncodeToString(privVal.Address),
		PubKey:     base64.StdEncoding.EncodeToString(BinaryBytes(privVal.PubKey)),
		PrivKey:    base64.StdEncoding.EncodeToString(BinaryBytes(privVal.PrivKey)),
		LastHeight: privVal.LastHeight,
		LastRound:  privVal.LastRound,
		LastStep:   privVal.LastStep,
	}
	privValJSONBytes, err := json.Marshal(privValJSON)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(privVal.filename, privValJSONBytes, 0700)
	if err != nil {
		panic(err)
	}
}

// TODO: test
func (privVal *PrivValidator) SignVote(vote *Vote) SignatureEd25519 {

	// If height regression, panic
	if privVal.LastHeight > vote.Height {
		panic("Height regression in SignVote")
	}
	// More cases for when the height matches
	if privVal.LastHeight == vote.Height {
		// If attempting any sign after commit, panic
		if privVal.LastStep == stepCommit {
			panic("SignVote on matching height after a commit")
		}
		// If round regression, panic
		if privVal.LastRound > vote.Round {
			panic("Round regression in SignVote")
		}
		// If step regression, panic
		if privVal.LastRound == vote.Round && privVal.LastStep > voteToStep(vote) {
			panic("Step regression in SignVote")
		}
	}

	// Persist height/round/step
	privVal.LastHeight = vote.Height
	privVal.LastRound = vote.Round
	privVal.LastStep = voteToStep(vote)
	privVal.Save()

	// Sign
	return privVal.SignVoteUnsafe(vote)
}

func (privVal *PrivValidator) SignVoteUnsafe(vote *Vote) SignatureEd25519 {
	return privVal.PrivKey.Sign(SignBytes(vote)).(SignatureEd25519)
}

func (privVal *PrivValidator) SignProposal(proposal *Proposal) SignatureEd25519 {
	if privVal.LastHeight < proposal.Height ||
		privVal.LastHeight == proposal.Height && privVal.LastRound < proposal.Round ||
		privVal.LastHeight == 0 && privVal.LastRound == 0 && privVal.LastStep == stepNone {

		// Persist height/round/step
		privVal.LastHeight = proposal.Height
		privVal.LastRound = proposal.Round
		privVal.LastStep = stepPropose
		privVal.Save()

		// Sign
		return privVal.PrivKey.Sign(SignBytes(proposal)).(SignatureEd25519)
	} else {
		panic(fmt.Sprintf("Attempt of duplicate signing of proposal: Height %v, Round %v", proposal.Height, proposal.Round))
	}
}

func (privVal *PrivValidator) SignRebondTx(rebondTx *RebondTx) SignatureEd25519 {
	if privVal.LastHeight < rebondTx.Height {

		// Persist height/round/step
		privVal.LastHeight = rebondTx.Height
		privVal.LastRound = math.MaxUint64 // We can't do anything else for this rebondTx.Height.
		privVal.LastStep = math.MaxUint8
		privVal.Save()

		// Sign
		return privVal.PrivKey.Sign(SignBytes(rebondTx)).(SignatureEd25519)
	} else {
		panic(fmt.Sprintf("Attempt of duplicate signing of rebondTx: Height %v", rebondTx.Height))
	}
}

func (privVal *PrivValidator) String() string {
	return fmt.Sprintf("PrivValidator{%X LH:%v, LR:%v, LS:%v}", privVal.Address, privVal.LastHeight, privVal.LastRound, privVal.LastStep)
}
