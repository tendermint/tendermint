package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

//-------------------------------------

// LastSignedInfo contains information about the latest
// data signed by a validator to help prevent double signing.
type LastSignedInfo struct {
	LastHeight    int64            `json:"last_height"`
	LastRound     int              `json:"last_round"`
	LastStep      int8             `json:"last_step"`
	LastSignature crypto.Signature `json:"last_signature,omitempty"` // so we dont lose signatures
	LastSignBytes data.Bytes       `json:"last_signbytes,omitempty"` // so we dont lose signatures

	saveFn func(CarefulSigner)
}

func NewLastSignedInfo(saveFn func(CarefulSigner)) *LastSignedInfo {
	return &LastSignedInfo{
		LastStep: -1,
		saveFn:   saveFn,
	}
}

func (info *LastSignedInfo) String() string {
	return fmt.Sprintf("LH:%v, LR:%v, LS:%v", info.LastHeight, info.LastRound, info.LastStep)
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements ConsensusSigner.
func (info *LastSignedInfo) SignVote(signer Signer, chainID string, vote *types.Vote) error {
	signature, err := info.sign(signer, vote.Height, vote.Round, voteToStep(vote),
		types.SignBytes(chainID, vote), checkVotesOnlyDifferByTimestamp)
	if err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	vote.Signature = signature
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements ConsensusSigner.
func (info *LastSignedInfo) SignProposal(signer Signer, chainID string, proposal *types.Proposal) error {
	signature, err := info.sign(signer, proposal.Height, proposal.Round, stepPropose,
		types.SignBytes(chainID, proposal), checkProposalsOnlyDifferByTimestamp)
	if err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	proposal.Signature = signature
	return nil
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements CosnensusSigner.
func (info *LastSignedInfo) SignHeartbeat(signer Signer, chainID string, heartbeat *types.Heartbeat) error {
	var err error
	heartbeat.Signature, err = signer.Sign(types.SignBytes(chainID, heartbeat))
	return err
}

// Verify returns an error if there is a height/round/step regression
// or if the HRS matches but there are no LastSignBytes.
// It returns true if HRS matches exactly and the LastSignature exists.
// It panics if the HRS matches, the LastSignBytes are not empty, but the LastSignature is empty.
func (info LastSignedInfo) Verify(height int64, round int, step int8) (bool, error) {
	if info.LastHeight > height {
		return false, errors.New("Height regression")
	}

	if info.LastHeight == height {
		if info.LastRound > round {
			return false, errors.New("Round regression")
		}

		if info.LastRound == round {
			if info.LastStep > step {
				return false, errors.New("Step regression")
			} else if info.LastStep == step {
				if info.LastSignBytes != nil {
					if info.LastSignature.Empty() {
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
func (info *LastSignedInfo) save(height int64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {

	info.LastHeight = height
	info.LastRound = round
	info.LastStep = step
	info.LastSignature = sig
	info.LastSignBytes = signBytes

	info.saveFn(info)
}

func (info *LastSignedInfo) Reset() {
	info.LastHeight = 0
	info.LastRound = 0
	info.LastStep = 0
	info.LastSignature = crypto.Signature{}
	info.LastSignBytes = nil
}

//-------------------------------------

// Sign signs the given signBytes with the signer if the height/round/step (HRS) are
// greater than the latest state of the LastSignedInfo. If the HRS are equal and the only thing changed is the timestamp,
// it returns the privValidator.LastSignature. Else it returns an error.
func (info *LastSignedInfo) sign(signer Signer, height int64, round int, step int8,
	signBytes []byte, checkFn checkOnlyDifferByTimestamp) (crypto.Signature, error) {

	sig := crypto.Signature{}

	sameHRS, err := info.Verify(height, round, step)
	if err != nil {
		return sig, err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS
	if sameHRS {
		// if they're the same or only differ by timestamp,
		// return the LastSignature. Otherwise, error
		if bytes.Equal(signBytes, info.LastSignBytes) ||
			checkFn(info.LastSignBytes, signBytes) {
			return info.LastSignature, nil
		}
		return sig, fmt.Errorf("Conflicting data")
	}

	// Sign
	sig, err = signer.Sign(signBytes)
	if err != nil {
		return sig, err
	}

	info.save(height, round, step, signBytes, sig)
	return sig, nil
}

//-------------------------------------

type checkOnlyDifferByTimestamp func([]byte, []byte) bool

// returns true if the only difference in the votes is their timestamp
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) bool {
	var lastVote, newVote types.CanonicalJSONOnceVote
	if err := json.Unmarshal(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	// set the times to the same value and check equality
	now := types.CanonicalTime(time.Now())
	lastVote.Vote.Timestamp = now
	newVote.Vote.Timestamp = now
	lastVoteBytes, _ := json.Marshal(lastVote)
	newVoteBytes, _ := json.Marshal(newVote)

	return bytes.Equal(newVoteBytes, lastVoteBytes)
}

// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) bool {
	var lastProposal, newProposal types.CanonicalJSONOnceProposal
	if err := json.Unmarshal(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := json.Unmarshal(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	// set the times to the same value and check equality
	now := types.CanonicalTime(time.Now())
	lastProposal.Proposal.Timestamp = now
	newProposal.Proposal.Timestamp = now
	lastProposalBytes, _ := json.Marshal(lastProposal)
	newProposalBytes, _ := json.Marshal(newProposal)

	return bytes.Equal(newProposalBytes, lastProposalBytes)
}
