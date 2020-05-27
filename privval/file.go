package privval

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/tempfile"
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

// A vote is either stepPrevote or stepPrecommit.
func voteToStep(vote *types.Vote) int8 {
	switch vote.Type {
	case types.PrevoteType:
		return stepPrevote
	case types.PrecommitType:
		return stepPrecommit
	default:
		panic(fmt.Sprintf("Unknown vote type: %v", vote.Type))
	}
}

//-------------------------------------------------------------------------------

// FilePVKey stores the immutable part of PrivValidator.
type FilePVKey struct {
	Address types.Address  `json:"address"`
	PubKey  crypto.PubKey  `json:"pub_key"`
	PrivKey crypto.PrivKey `json:"priv_key"`

	filePath string
}

// Save persists the FilePVKey to its filePath.
func (pvKey FilePVKey) Save() {
	outFile := pvKey.filePath
	if outFile == "" {
		panic("cannot save PrivValidator key: filePath not set")
	}

	jsonBytes, err := cdc.MarshalJSONIndent(pvKey, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}

}

//-------------------------------------------------------------------------------

// FilePVLastSignState stores the mutable part of PrivValidator.
type FilePVLastSignState struct {
	Height    int64            `json:"height"`
	Round     int              `json:"round"`
	Step      int8             `json:"step"`
	Signature []byte           `json:"signature,omitempty"`
	SignBytes tmbytes.HexBytes `json:"signbytes,omitempty"`

	filePath string
}

// CheckHRS checks the given height, round, step (HRS) against that of the
// FilePVLastSignState. It returns an error if the arguments constitute a regression,
// or if they match but the SignBytes are empty.
// The returned boolean indicates whether the last Signature should be reused -
// it returns true if the HRS matches the arguments and the SignBytes are not empty (indicating
// we have already signed for this HRS, and can reuse the existing signature).
// It panics if the HRS matches the arguments, there's a SignBytes, but no Signature.
func (lss *FilePVLastSignState) CheckHRS(height int64, round int, step int8) (bool, error) {

	if lss.Height > height {
		return false, fmt.Errorf("height regression. Got %v, last height %v", height, lss.Height)
	}

	if lss.Height == height {
		if lss.Round > round {
			return false, fmt.Errorf("round regression at height %v. Got %v, last round %v", height, round, lss.Round)
		}

		if lss.Round == round {
			if lss.Step > step {
				return false, fmt.Errorf(
					"step regression at height %v round %v. Got %v, last step %v",
					height,
					round,
					step,
					lss.Step,
				)
			} else if lss.Step == step {
				if lss.SignBytes != nil {
					if lss.Signature == nil {
						panic("pv: Signature is nil but SignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("no SignBytes found")
			}
		}
	}
	return false, nil
}

// Save persists the FilePvLastSignState to its filePath.
func (lss *FilePVLastSignState) Save() {
	outFile := lss.filePath
	if outFile == "" {
		panic("cannot save FilePVLastSignState: filePath not set")
	}
	jsonBytes, err := cdc.MarshalJSONIndent(lss, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

//-------------------------------------------------------------------------------

// FilePV implements PrivValidator using data persisted to disk
// to prevent double signing.
// NOTE: the directories containing pv.Key.filePath and pv.LastSignState.filePath must already exist.
// It includes the LastSignature and LastSignBytes so we don't lose the signature
// if the process crashes after signing but before the resulting consensus message is processed.
type FilePV struct {
	Key           FilePVKey
	LastSignState FilePVLastSignState
}

// GenFilePV generates a new validator with randomly generated private key
// and sets the filePaths, but does not call Save().
func GenFilePV(keyFilePath, stateFilePath string) *FilePV {
	privKey := ed25519.GenPrivKey()

	return &FilePV{
		Key: FilePVKey{
			Address:  privKey.PubKey().Address(),
			PubKey:   privKey.PubKey(),
			PrivKey:  privKey,
			filePath: keyFilePath,
		},
		LastSignState: FilePVLastSignState{
			Step:     stepNone,
			filePath: stateFilePath,
		},
	}
}

// LoadFilePV loads a FilePV from the filePaths.  The FilePV handles double
// signing prevention by persisting data to the stateFilePath.  If either file path
// does not exist, the program will exit.
func LoadFilePV(keyFilePath, stateFilePath string) *FilePV {
	return loadFilePV(keyFilePath, stateFilePath, true)
}

// LoadFilePVEmptyState loads a FilePV from the given keyFilePath, with an empty LastSignState.
// If the keyFilePath does not exist, the program will exit.
func LoadFilePVEmptyState(keyFilePath, stateFilePath string) *FilePV {
	return loadFilePV(keyFilePath, stateFilePath, false)
}

// If loadState is true, we load from the stateFilePath. Otherwise, we use an empty LastSignState.
func loadFilePV(keyFilePath, stateFilePath string, loadState bool) *FilePV {
	keyJSONBytes, err := ioutil.ReadFile(keyFilePath)
	if err != nil {
		tmos.Exit(err.Error())
	}
	pvKey := FilePVKey{}
	err = cdc.UnmarshalJSON(keyJSONBytes, &pvKey)
	if err != nil {
		tmos.Exit(fmt.Sprintf("Error reading PrivValidator key from %v: %v\n", keyFilePath, err))
	}

	// overwrite pubkey and address for convenience
	pvKey.PubKey = pvKey.PrivKey.PubKey()
	pvKey.Address = pvKey.PubKey.Address()
	pvKey.filePath = keyFilePath

	pvState := FilePVLastSignState{}
	if loadState {
		stateJSONBytes, err := ioutil.ReadFile(stateFilePath)
		if err != nil {
			tmos.Exit(err.Error())
		}
		err = cdc.UnmarshalJSON(stateJSONBytes, &pvState)
		if err != nil {
			tmos.Exit(fmt.Sprintf("Error reading PrivValidator state from %v: %v\n", stateFilePath, err))
		}
	}

	pvState.filePath = stateFilePath

	return &FilePV{
		Key:           pvKey,
		LastSignState: pvState,
	}
}

// LoadOrGenFilePV loads a FilePV from the given filePaths
// or else generates a new one and saves it to the filePaths.
func LoadOrGenFilePV(keyFilePath, stateFilePath string) *FilePV {
	var pv *FilePV
	if tmos.FileExists(keyFilePath) {
		pv = LoadFilePV(keyFilePath, stateFilePath)
	} else {
		pv = GenFilePV(keyFilePath, stateFilePath)
		pv.Save()
	}
	return pv
}

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetAddress() types.Address {
	return pv.Key.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey() (crypto.PubKey, error) {
	return pv.Key.PubKey, nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVote(chainID string, vote *types.Vote) error {
	if err := pv.signVote(chainID, vote); err != nil {
		return fmt.Errorf("error signing vote: %v", err)
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *FilePV) SignProposal(chainID string, proposal *types.Proposal) error {
	if err := pv.signProposal(chainID, proposal); err != nil {
		return fmt.Errorf("error signing proposal: %v", err)
	}
	return nil
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() {
	pv.Key.Save()
	pv.LastSignState.Save()
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() {
	var sig []byte
	pv.LastSignState.Height = 0
	pv.LastSignState.Round = 0
	pv.LastSignState.Step = 0
	pv.LastSignState.Signature = sig
	pv.LastSignState.SignBytes = nil
	pv.Save()
}

// String returns a string representation of the FilePV.
func (pv *FilePV) String() string {
	return fmt.Sprintf(
		"PrivValidator{%v LH:%v, LR:%v, LS:%v}",
		pv.GetAddress(),
		pv.LastSignState.Height,
		pv.LastSignState.Round,
		pv.LastSignState.Step,
	)
}

//------------------------------------------------------------------------------------

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(chainID string, vote *types.Vote) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)

	lss := pv.LastSignState

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	signBytes := vote.SignBytes(chainID)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, lss.SignBytes) {
			vote.Signature = lss.Signature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(lss.SignBytes, signBytes); ok {
			vote.Timestamp = timestamp
			vote.Signature = lss.Signature
		} else {
			err = fmt.Errorf("conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the vote
	sig, err := pv.Key.PrivKey.Sign(signBytes)
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

	lss := pv.LastSignState

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	signBytes := proposal.SignBytes(chainID)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(signBytes, lss.SignBytes) {
			proposal.Signature = lss.Signature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(lss.SignBytes, signBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = lss.Signature
		} else {
			err = fmt.Errorf("conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the proposal
	sig, err := pv.Key.PrivKey.Sign(signBytes)
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

	pv.LastSignState.Height = height
	pv.LastSignState.Round = round
	pv.LastSignState.Step = step
	pv.LastSignState.Signature = sig
	pv.LastSignState.SignBytes = signBytes
	pv.LastSignState.Save()
}

//-----------------------------------------------------------------------------------------

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the votes is their timestamp.
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastVote, newVote types.CanonicalVote
	if err := cdc.UnmarshalBinaryLengthPrefixed(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := cdc.UnmarshalBinaryLengthPrefixed(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	lastTime := lastVote.Timestamp

	// set the times to the same value and check equality
	now := tmtime.Now()
	lastVote.Timestamp = now
	newVote.Timestamp = now
	lastVoteBytes, _ := cdc.MarshalJSON(lastVote)
	newVoteBytes, _ := cdc.MarshalJSON(newVote)

	return lastTime, bytes.Equal(newVoteBytes, lastVoteBytes)
}

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal types.CanonicalProposal
	if err := cdc.UnmarshalBinaryLengthPrefixed(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := cdc.UnmarshalBinaryLengthPrefixed(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime := lastProposal.Timestamp
	// set the times to the same value and check equality
	now := tmtime.Now()
	lastProposal.Timestamp = now
	newProposal.Timestamp = now
	lastProposalBytes, _ := cdc.MarshalBinaryLengthPrefixed(lastProposal)
	newProposalBytes, _ := cdc.MarshalBinaryLengthPrefixed(newProposal)

	return lastTime, bytes.Equal(newProposalBytes, lastProposalBytes)
}
