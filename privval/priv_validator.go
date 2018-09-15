package privval

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
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
		cmn.PanicSanity("Unknown vote type")
		return 0
	}
}

// FilePV implements PrivValidator using data persisted to disk
// to prevent double signing.
// NOTE: the directory containing the pv.filePath must already exist.
type FilePV struct {
	Address       types.Address  `json:"address"`
	PubKey        crypto.PubKey  `json:"pub_key"`
	LastHeight    int64          `json:"last_height"`
	LastRound     int            `json:"last_round"`
	LastStep      int8           `json:"last_step"`
	LastSignature []byte         `json:"last_signature,omitempty"` // so we dont lose signatures XXX Why would we lose signatures?
	LastSignBytes cmn.HexBytes   `json:"last_signbytes,omitempty"` // so we dont lose signatures XXX Why would we lose signatures?
	PrivKey       crypto.PrivKey `json:"priv_key"`

	// For persistence.
	// Overloaded for testing.
	filePath string
	mtx      sync.Mutex
}

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetAddress() types.Address {
	return pv.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey() crypto.PubKey {
	return pv.PubKey
}

// GenFilePV generates a new validator with randomly generated private key
// and sets the filePath, but does not call Save().
func GenFilePV(filePath string) *FilePV {
	privKey := ed25519.GenPrivKey()
	return &FilePV{
		Address:  privKey.PubKey().Address(),
		PubKey:   privKey.PubKey(),
		PrivKey:  privKey,
		LastStep: stepNone,
		filePath: filePath,
	}
}

// LoadFilePV loads a FilePV from the filePath.  The FilePV handles double
// signing prevention by persisting data to the filePath.  If the filePath does
// not exist, the FilePV must be created manually and saved.
func LoadFilePV(filePath string) *FilePV {
	pvJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	pv := &FilePV{}
	err = cdc.UnmarshalJSON(pvJSONBytes, &pv)
	if err != nil {
		cmn.Exit(fmt.Sprintf("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	// overwrite pubkey and address for convenience
	pv.PubKey = pv.PrivKey.PubKey()
	pv.Address = pv.PubKey.Address()

	pv.filePath = filePath
	return pv
}

// LoadOrGenFilePV loads a FilePV from the given filePath
// or else generates a new one and saves it to the filePath.
func LoadOrGenFilePV(filePath string) *FilePV {
	var pv *FilePV
	if cmn.FileExists(filePath) {
		pv = LoadFilePV(filePath)
	} else {
		pv = GenFilePV(filePath)
		pv.Save()
	}
	return pv
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	pv.save()
}

func (pv *FilePV) save() {
	outFile := pv.filePath
	if outFile == "" {
		panic("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes, err := cdc.MarshalJSONIndent(pv, "", "  ")
	if err != nil {
		panic(err)
	}
	err = cmn.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() {
	var sig []byte
	pv.LastHeight = 0
	pv.LastRound = 0
	pv.LastStep = 0
	pv.LastSignature = sig
	pv.LastSignBytes = nil
	pv.Save()
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVote(chainID string, vote *types.Vote) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if err := pv.signVote(chainID, vote); err != nil {
		return fmt.Errorf("Error signing vote: %v", err)
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *FilePV) SignProposal(chainID string, proposal *types.Proposal) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	if err := pv.signProposal(chainID, proposal); err != nil {
		return fmt.Errorf("Error signing proposal: %v", err)
	}
	return nil
}

// returns error if HRS regression or no LastSignBytes. returns true if HRS is unchanged
func (pv *FilePV) checkHRS(height int64, round int, step int8) (bool, error) {
	if pv.LastHeight > height {
		return false, errors.New("Height regression")
	}

	if pv.LastHeight == height {
		if pv.LastRound > round {
			return false, errors.New("Round regression")
		}

		if pv.LastRound == round {
			if pv.LastStep > step {
				return false, errors.New("Step regression")
			} else if pv.LastStep == step {
				if pv.LastSignBytes != nil {
					if pv.LastSignature == nil {
						panic("pv: LastSignature is nil but LastSignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("No LastSignature found")
			}
		}
	}
	return false, nil
}

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(chainID string, vote *types.Vote) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)
	signBytes := vote.SignBytes(chainID)

	sameHRS, err := pv.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.LastSignBytes) {
			vote.Signature = pv.LastSignature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(pv.LastSignBytes, signBytes); ok {
			vote.Timestamp = timestamp
			vote.Signature = pv.LastSignature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the vote
	sig, err := pv.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	vote.Signature = sig
	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *FilePV) signProposal(chainID string, proposal *types.Proposal) error {
	height, round, step := proposal.Height, proposal.Round, stepPropose
	signBytes := proposal.SignBytes(chainID)

	sameHRS, err := pv.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, pv.LastSignBytes) {
			proposal.Signature = pv.LastSignature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(pv.LastSignBytes, signBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = pv.LastSignature
		} else {
			err = fmt.Errorf("Conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the proposal
	sig, err := pv.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	proposal.Signature = sig
	return nil
}

// Persist height/round/step and signature
func (pv *FilePV) saveSigned(height int64, round int, step int8,
	signBytes []byte, sig []byte) {

	pv.LastHeight = height
	pv.LastRound = round
	pv.LastStep = step
	pv.LastSignature = sig
	pv.LastSignBytes = signBytes
	pv.save()
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements PrivValidator.
func (pv *FilePV) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	sig, err := pv.PrivKey.Sign(heartbeat.SignBytes(chainID))
	if err != nil {
		return err
	}
	heartbeat.Signature = sig
	return nil
}

// String returns a string representation of the FilePV.
func (pv *FilePV) String() string {
	return fmt.Sprintf("PrivValidator{%v LH:%v, LR:%v, LS:%v}", pv.GetAddress(), pv.LastHeight, pv.LastRound, pv.LastStep)
}

//-------------------------------------

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the votes is their timestamp.
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastVote, newVote types.CanonicalJSONVote
	if err := cdc.UnmarshalJSON(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := cdc.UnmarshalJSON(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	lastTime, err := time.Parse(types.TimeFormat, lastVote.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := types.CanonicalTime(tmtime.Now())
	lastVote.Timestamp = now
	newVote.Timestamp = now
	lastVoteBytes, _ := cdc.MarshalJSON(lastVote)
	newVoteBytes, _ := cdc.MarshalJSON(newVote)

	return lastTime, bytes.Equal(newVoteBytes, lastVoteBytes)
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal types.CanonicalJSONProposal
	if err := cdc.UnmarshalJSON(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := cdc.UnmarshalJSON(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime, err := time.Parse(types.TimeFormat, lastProposal.Timestamp)
	if err != nil {
		panic(err)
	}

	// set the times to the same value and check equality
	now := types.CanonicalTime(tmtime.Now())
	lastProposal.Timestamp = now
	newProposal.Timestamp = now
	lastProposalBytes, _ := cdc.MarshalJSON(lastProposal)
	newProposalBytes, _ := cdc.MarshalJSON(newProposal)

	return lastTime, bytes.Equal(newProposalBytes, lastProposalBytes)
}
