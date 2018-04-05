package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

// TODO: type ?
const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
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

//-------------------------------------

// LastSignedInfo contains information about the latest
// data signed by a validator to help prevent double signing.
type LastSignedInfo struct {
	Height    int64            `json:"height"`
	Round     int              `json:"round"`
	Step      int8             `json:"step"`
	Signature crypto.Signature `json:"signature,omitempty"` // so we dont lose signatures
	SignBytes cmn.HexBytes     `json:"signbytes,omitempty"` // so we dont lose signatures
}

func NewLastSignedInfo() *LastSignedInfo {
	return &LastSignedInfo{
		Step: stepNone,
	}
}

func (lsi *LastSignedInfo) String() string {
	return fmt.Sprintf("LH:%v, LR:%v, LS:%v", lsi.Height, lsi.Round, lsi.Step)
}

// Verify returns an error if there is a height/round/step regression
// or if the HRS matches but there are no LastSignBytes.
// It returns true if HRS matches exactly and the LastSignature exists.
// It panics if the HRS matches, the LastSignBytes are not empty, but the LastSignature is empty.
func (lsi LastSignedInfo) Verify(height int64, round int, step int8) (bool, error) {
	if lsi.Height > height {
		return false, errors.New("Height regression")
	}

	if lsi.Height == height {
		if lsi.Round > round {
			return false, errors.New("Round regression")
		}

		if lsi.Round == round {
			if lsi.Step > step {
				return false, errors.New("Step regression")
			} else if lsi.Step == step {
				if lsi.SignBytes != nil {
					if lsi.Signature.Empty() {
						panic("info: LastSignature is nil but LastSignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("No LastSignature found")
			}
		}
	}
	return false, nil
}

// Set height/round/step and signature on the info
func (lsi *LastSignedInfo) Set(height int64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {

	lsi.Height = height
	lsi.Round = round
	lsi.Step = step
	lsi.Signature = sig
	lsi.SignBytes = signBytes
}

// Reset resets all the values.
// XXX: Unsafe.
func (lsi *LastSignedInfo) Reset() {
	lsi.Height = 0
	lsi.Round = 0
	lsi.Step = 0
	lsi.Signature = crypto.Signature{}
	lsi.SignBytes = nil
}

// SignVote checks the height/round/step (HRS) are greater than the latest state of the LastSignedInfo.
// If so, it signs the vote, updates the LastSignedInfo, and sets the signature on the vote.
// If the HRS are equal and the only thing changed is the timestamp, it sets the vote.Timestamp to the previous
// value and the Signature to the LastSignedInfo.Signature.
// Else it returns an error.
func (lsi *LastSignedInfo) SignVote(signer types.Signer, chainID string, vote *types.Vote) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)
	signBytes := vote.SignBytes(chainID)

	sameHRS, err := lsi.Verify(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, lsi.SignBytes) {
			vote.Signature = lsi.Signature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(lsi.SignBytes, signBytes); ok {
			vote.Timestamp = timestamp
			vote.Signature = lsi.Signature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}
	sig, err := signer.Sign(signBytes)
	if err != nil {
		return err
	}
	lsi.Set(height, round, step, signBytes, sig)
	vote.Signature = sig
	return nil
}

// SignProposal checks if the height/round/step (HRS) are greater than the latest state of the LastSignedInfo.
// If so, it signs the proposal, updates the LastSignedInfo, and sets the signature on the proposal.
// If the HRS are equal and the only thing changed is the timestamp, it sets the timestamp to the previous
// value and the Signature to the LastSignedInfo.Signature.
// Else it returns an error.
func (lsi *LastSignedInfo) SignProposal(signer types.Signer, chainID string, proposal *types.Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose
	signBytes := proposal.SignBytes(chainID)

	sameHRS, err := lsi.Verify(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, lsi.SignBytes) {
			proposal.Signature = lsi.Signature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(lsi.SignBytes, signBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = lsi.Signature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}
	sig, err := signer.Sign(signBytes)
	if err != nil {
		return err
	}
	lsi.Set(height, round, step, signBytes, sig)
	proposal.Signature = sig
	return nil
}

//-------------------------------------

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the votes is their timestamp.
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastVote, newVote types.CanonicalJSONOnceVote
	if err := json.Unmarshal(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	lastTime, err := time.Parse(types.TimeFormat, lastVote.Vote.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := types.CanonicalTime(time.Now())
	lastVote.Vote.Timestamp = now
	newVote.Vote.Timestamp = now
	lastVoteBytes, _ := json.Marshal(lastVote)
	newVoteBytes, _ := json.Marshal(newVote)

	return lastTime, bytes.Equal(newVoteBytes, lastVoteBytes)
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal types.CanonicalJSONOnceProposal
	if err := json.Unmarshal(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime, err := time.Parse(types.TimeFormat, lastProposal.Proposal.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := types.CanonicalTime(time.Now())
	lastProposal.Proposal.Timestamp = now
	newProposal.Proposal.Timestamp = now
	lastProposalBytes, _ := json.Marshal(lastProposal)
	newProposalBytes, _ := json.Marshal(newProposal)

	return lastTime, bytes.Equal(newProposalBytes, lastProposalBytes)
}
