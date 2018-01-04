package types

import (
	"bytes"
	"errors"
	"fmt"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

var _ CarefulSigner = (*DefaultCarefulSigner)(nil)

type DefaultCarefulSigner struct {
	LastSignedInfo *LastSignedInfo `json:"last_signed_info"`

	saveFn func(LastSignedInfo)
}

func NewDefaultCarefulSigner(saveFn func(LastSignedInfo)) *DefaultCarefulSigner {
	return &DefaultCarefulSigner{
		LastSignedInfo: NewLastSignedInfo(),
		saveFn:         saveFn,
	}
}

func (dcs *DefaultCarefulSigner) String() string {
	return dcs.LastSignedInfo.String()
}

func (dcs *DefaultCarefulSigner) Reset() {
	dcs.LastSignedInfo.Reset()
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. It sets the signature on the vote. If a previous vote was signed
// that is identical except for the timestamp, it sets the timestamp to the previous value.
// Implements CarefulSigner.
func (dcs *DefaultCarefulSigner) SignVote(signer Signer, chainID string, vote *types.Vote) error {
	if err := dcs.signVote(signer, chainID, vote); err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. It sets the signature on the proposal. If a previous proposal was signed
// that is identical except for the timestamp, it sets the timestamp to the previous value.
// Implements CarefulSigner.
func (dcs *DefaultCarefulSigner) SignProposal(signer Signer, chainID string, proposal *types.Proposal) error {
	if err := dcs.signProposal(signer, chainID, proposal); err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	return nil
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements CarefulSigner.
func (dcs *DefaultCarefulSigner) SignHeartbeat(signer Signer, chainID string, heartbeat *types.Heartbeat) error {
	var err error
	heartbeat.Signature, err = signer.Sign(types.SignBytes(chainID, heartbeat))
	return err
}

// signVote signs the given vote with the signer and sets the vote.Signature if the height/round/step (HRS) are
// greater than the latest state of the LastSignedInfo. If the HRS are equal and the only thing changed
// is the timestamp, it sets the timestamp to the previous value and the Signature to the LastSignedInfo.Signature.
// Else it returns an error.
func (dcs *DefaultCarefulSigner) signVote(signer Signer, chainID string, vote *types.Vote) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)
	signBytes := types.SignBytes(chainID, vote)

	sameHRS, err := dcs.LastSignedInfo.Verify(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, dcs.LastSignedInfo.SignBytes) {
			vote.Signature = dcs.LastSignedInfo.Signature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(dcs.LastSignedInfo.SignBytes, signBytes); ok {
			vote.Timestamp = timestamp
			vote.Signature = dcs.LastSignedInfo.Signature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the vote
	sig, err := signer.Sign(signBytes)
	if err != nil {
		return err
	}
	dcs.save(height, round, step, signBytes, sig)
	vote.Signature = sig
	return nil
}

// signProposal signs the given proposal with the signer and sets the proposal.Signature if the height/round/step (HRS) are
// greater than the latest state of the LastSignedInfo. If the HRS are equal and the only thing changed
// is the timestamp, it sets the timestamp to the previous value and the Signature to the LastSignedInfo.Signature.
// Else it returns an error.
func (dcs *DefaultCarefulSigner) signProposal(signer Signer, chainID string, proposal *types.Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose
	signBytes := types.SignBytes(chainID, proposal)

	sameHRS, err := dcs.LastSignedInfo.Verify(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, dcs.LastSignedInfo.SignBytes) {
			proposal.Signature = dcs.LastSignedInfo.Signature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(dcs.LastSignedInfo.SignBytes, signBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = dcs.LastSignedInfo.Signature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the proposal
	sig, err := signer.Sign(signBytes)
	if err != nil {
		return err
	}
	dcs.save(height, round, step, signBytes, sig)
	proposal.Signature = sig
	return nil
}

func (dcs *DefaultCarefulSigner) save(height int64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {
	dcs.LastSignedInfo.Set(height, round, step, signBytes, sig)
	dcs.saveFn(*dcs.LastSignedInfo)
}

//-------------------------------------
