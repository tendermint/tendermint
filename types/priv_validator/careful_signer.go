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
// chainID. Implements CarefulSigner.
func (dcs *DefaultCarefulSigner) SignVote(signer Signer, chainID string, vote *types.Vote) error {
	signature, err := dcs.sign(signer, vote.Height, vote.Round, voteToStep(vote),
		types.SignBytes(chainID, vote), checkVotesOnlyDifferByTimestamp)
	if err != nil {
		return errors.New(cmn.Fmt("Error signing vote: %v", err))
	}
	vote.Signature = signature
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements CarefulSigner.
func (dcs *DefaultCarefulSigner) SignProposal(signer Signer, chainID string, proposal *types.Proposal) error {
	signature, err := dcs.sign(signer, proposal.Height, proposal.Round, stepPropose,
		types.SignBytes(chainID, proposal), checkProposalsOnlyDifferByTimestamp)
	if err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	proposal.Signature = signature
	return nil
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements CarefulSigner.
func (dcs *DefaultCarefulSigner) SignHeartbeat(signer Signer, chainID string, heartbeat *types.Heartbeat) error {
	var err error
	heartbeat.Signature, err = signer.Sign(types.SignBytes(chainID, heartbeat))
	return err
}

// Sign signs the given signBytes with the signer if the height/round/step (HRS) are
// greater than the latest state of the LastSignedInfo.
// If the HRS are equal and the only thing changed is the timestamp,
// it returns the LastSignature. Else it returns an error.
func (dcs *DefaultCarefulSigner) sign(signer Signer, height int64, round int, step int8,
	signBytes []byte, checkFn checkOnlyDifferByTimestamp) (crypto.Signature, error) {

	sig := crypto.Signature{}

	sameHRS, err := dcs.LastSignedInfo.Verify(height, round, step)
	if err != nil {
		return sig, err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS
	if sameHRS {
		// if they're the same or only differ by timestamp,
		// return the LastSignature. Otherwise, error
		if bytes.Equal(signBytes, dcs.LastSignedInfo.SignBytes) ||
			checkFn(dcs.LastSignedInfo.SignBytes, signBytes) {
			return dcs.LastSignedInfo.Signature, nil
		}
		return sig, fmt.Errorf("Conflicting data")
	}

	// Sign
	sig, err = signer.Sign(signBytes)
	if err != nil {
		return sig, err
	}

	dcs.save(height, round, step, signBytes, sig)
	return sig, nil
}

func (dcs *DefaultCarefulSigner) save(height int64, round int, step int8,
	signBytes []byte, sig crypto.Signature) {
	dcs.LastSignedInfo.Set(height, round, step, signBytes, sig)
	dcs.saveFn(*dcs.LastSignedInfo)
}

//-------------------------------------
