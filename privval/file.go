package privval

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/protoio"
	"github.com/tendermint/tendermint/libs/tempfile"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
func voteToStep(vote *tmproto.Vote) int8 {
	switch vote.Type {
	case tmproto.PrevoteType:
		return stepPrevote
	case tmproto.PrecommitType:
		return stepPrecommit
	default:
		panic(fmt.Sprintf("Unknown vote type: %v", vote.Type))
	}
}

//-------------------------------------------------------------------------------

// FilePVKey stores the immutable part of PrivValidator.
type FilePVKey struct {
	PrivateKeys          map[string]crypto.QuorumKeys `json:"private_keys"`
	// heightString -> quorumHash
	UpdateHeights        map[string]crypto.QuorumHash `json:"update_heights"`
	// quorumHash -> heightString
	FirstHeightOfQuorums map[string]string `json:"first_height_of_quorums"`
	ProTxHash            crypto.ProTxHash `json:"pro_tx_hash"`

	filePath string
}

// Save persists the FilePVKey to its filePath.
func (pvKey FilePVKey) Save() {
	outFile := pvKey.filePath
	if outFile == "" {
		panic("cannot save PrivValidator key: filePath not set")
	}

	jsonBytes, err := tmjson.MarshalIndent(pvKey, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}

}

func (pvKey FilePVKey) PrivateKeyForQuorumHash(quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	if keys, ok := pvKey.PrivateKeys[quorumHash.String()]; ok {
		return keys.PrivKey, nil
	}
	return nil, fmt.Errorf("no threshold public key for quorum hash %v", quorumHash)
}

func (pvKey FilePVKey) PublicKeyForQuorumHash(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if keys, ok := pvKey.PrivateKeys[quorumHash.String()]; ok {
		return keys.PubKey, nil
	}
	return nil, fmt.Errorf("no threshold public key for quorum hash %v", quorumHash)
}

func (pvKey FilePVKey) ThresholdPublicKeyForQuorumHash(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if keys, ok := pvKey.PrivateKeys[quorumHash.String()]; ok {
		return keys.ThresholdPublicKey, nil
	}
	return nil, fmt.Errorf("no threshold public key for quorum hash %v", quorumHash)
}


//-------------------------------------------------------------------------------

// FilePVLastSignState stores the mutable part of PrivValidator.
type FilePVLastSignState struct {
	Height         int64            `json:"height"`
	Round          int32            `json:"round"`
	Step           int8             `json:"step"`
	BlockSignature []byte           `json:"block_signature,omitempty"`
	BlockSignBytes tmbytes.HexBytes `json:"block_sign_bytes,omitempty"`
	StateSignature []byte           `json:"state_signature,omitempty"`
	StateSignBytes tmbytes.HexBytes `json:"state_sign_bytes,omitempty"`

	filePath string
}

// CheckHRS checks the given height, round, step (HRS) against that of the
// FilePVLastSignState. It returns an error if the arguments constitute a regression,
// or if they match but the SignBytes are empty.
// The returned boolean indicates whether the last Signature should be reused -
// it returns true if the HRS matches the arguments and the SignBytes are not empty (indicating
// we have already signed for this HRS, and can reuse the existing signature).
// It panics if the HRS matches the arguments, there's a SignBytes, but no Signature.
func (lss *FilePVLastSignState) CheckHRS(height int64, round int32, step int8) (bool, error) {

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
				if lss.BlockSignBytes != nil {
					if lss.BlockSignature == nil {
						panic("pv: BlockID Signature is nil but BlockSignBytes is not!")
					}
					return true, nil
				}
				if lss.StateSignBytes != nil {
					if lss.StateSignature == nil {
						panic("pv: StateID Signature is nil but StateSignBytes is not!")
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
	jsonBytes, err := tmjson.MarshalIndent(lss, "", "  ")
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

// FilePVOption ...
type FilePVOption func(filePV *FilePV) error

// NewFilePVOneKey generates a new validator from the given key and paths.
func NewFilePVOneKey(privKey crypto.PrivKey, proTxHash []byte, quorumHash crypto.QuorumHash, thresholdPublicKey crypto.PubKey,
	keyFilePath, stateFilePath string) *FilePV {
	if len(proTxHash) != crypto.ProTxHashSize {
		panic("error setting incorrect proTxHash size in NewFilePV")
	}

	quorumKeys := crypto.QuorumKeys{
		PrivKey: privKey,
		PubKey: privKey.PubKey(),
		ThresholdPublicKey: thresholdPublicKey,
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	return &FilePV{
		Key: FilePVKey{
			PrivateKeys:        privateKeysMap,
			ProTxHash:          proTxHash,
			filePath:           keyFilePath,
		},
		LastSignState: FilePVLastSignState{
			Step:     stepNone,
			filePath: stateFilePath,
		},
	}
}

// WithKeyAndStateFilePaths ...
func WithKeyAndStateFilePaths(keyFilePath, stateFilePath string) FilePVOption {
	return func(filePV *FilePV) error {
		filePV.Key.filePath = keyFilePath
		filePV.LastSignState.filePath = stateFilePath
		return nil
	}
}

// WithPrivateKeys ...
func WithPrivateKeys(keys []crypto.PrivKey, quorumHashes []crypto.QuorumHash, thresholdPublicKeys *[]crypto.PubKey) FilePVOption {
	return func(filePV *FilePV) error {
		if len(keys) == 0 {
			return errors.New("there must be at least one key")
		}
		if len(keys) != len(quorumHashes) {
			return errors.New("keys must be same length as quorumHashes")
		}
		if thresholdPublicKeys != nil && len(*thresholdPublicKeys) != len(quorumHashes) {
			return errors.New("thresholdPublicKeys must be same length as quorumHashes and keys if they are set")
		}

		privateKeysMap := make(map[string]crypto.QuorumKeys)
		for i, key := range keys {
			quorumKeys := crypto.QuorumKeys{
				PrivKey: key,
				PubKey: key.PubKey(),
			}
			if thresholdPublicKeys != nil {
				quorumKeys.ThresholdPublicKey = (*thresholdPublicKeys)[i]
			}
			privateKeysMap[quorumHashes[i].String()] = quorumKeys
		}
		return nil
	}
}

func WithUpdateHeights(updateHeights map[int64]crypto.QuorumHash) FilePVOption {
	return func(filePV *FilePV) error {
		filePV.Key.UpdateHeights = make(map[string]crypto.QuorumHash)
		for height, quorumHash := range updateHeights {
			filePV.Key.UpdateHeights[strconv.Itoa(int(height))] = quorumHash

			if _, ok := filePV.Key.FirstHeightOfQuorums[quorumHash.String()]; ok {
				filePV.Key.FirstHeightOfQuorums[quorumHash.String()] = strconv.Itoa(int(height))
			}
		}
		return nil
	}
}

// WithProTxHash ...
func WithProTxHash(proTxHash []byte) FilePVOption {
	return func(filePV *FilePV) error {
		if len(proTxHash) != crypto.ProTxHashSize {
			return fmt.Errorf("error setting incorrect proTxHash size in NewFilePV")
		}
		filePV.Key.ProTxHash = proTxHash
		return nil
	}
}

// NewFilePVWithOptions ...
func NewFilePVWithOptions(opts ...FilePVOption) (*FilePV, error) {
	filePV := &FilePV{}
	for _, opt := range opts {
		err := opt(filePV)
		if err != nil {
			return nil, err
		}
	}
	return filePV, nil
}

// GenFilePV generates a new validator with randomly generated private key
// and sets the filePaths, but does not call Save().
func GenFilePV(keyFilePath, stateFilePath string) *FilePV {
	return NewFilePVOneKey(bls12381.GenPrivKey(), crypto.RandProTxHash(), crypto.RandQuorumHash(), nil,
		keyFilePath, stateFilePath)
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
	err = tmjson.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		tmos.Exit(fmt.Sprintf("Error reading PrivValidator key from %v: %v\n", keyFilePath, err))
	}
	// verify proTxHash is 32 bytes if it exists
	if pvKey.ProTxHash != nil && len(pvKey.ProTxHash) != crypto.ProTxHashSize {
		tmos.Exit(fmt.Sprintf("loadFilePV proTxHash must be 32 bytes in key file path %s", keyFilePath))
	}

	pvKey.filePath = keyFilePath

	pvState := FilePVLastSignState{}

	if loadState {
		stateJSONBytes, err := ioutil.ReadFile(stateFilePath)
		if err != nil {
			tmos.Exit(err.Error())
		}
		err = tmjson.Unmarshal(stateJSONBytes, &pvState)
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

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if keys, ok := pv.Key.PrivateKeys[quorumHash.String()]; ok {
		return keys.PubKey, nil
	}
	return nil, fmt.Errorf("no public key for quorum hash %v", quorumHash)
}

// GetFirstPubKey returns the first public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetFirstPubKey() (crypto.PubKey, error) {
	for _, quorumKeys := range pv.Key.PrivateKeys {
		return quorumKeys.PubKey, nil
	}
	return nil, nil
}

func (pv *FilePV) GetQuorumHashes() ([]crypto.QuorumHash, error) {
	quorumHashes := make([]crypto.QuorumHash, len(pv.Key.PrivateKeys))
	i := 0
	for quorumHashString, _ := range pv.Key.PrivateKeys {
		quorumHash, err := hex.DecodeString(quorumHashString)
		quorumHashes[i] = quorumHash
		if err != nil {
			return nil, err
		}
		i++
	}
	return quorumHashes, nil
}

func (pv *FilePV) GetFirstQuorumHash() (crypto.QuorumHash, error) {
	for quorumHashString, _ := range pv.Key.PrivateKeys {
		return hex.DecodeString(quorumHashString)
	}
	return nil, nil
}

// GetThresholdPublicKey ...
func (pv *FilePV) GetThresholdPublicKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if keys, ok := pv.Key.PrivateKeys[quorumHash.String()]; ok {
		return keys.ThresholdPublicKey, nil
	}
	return nil, fmt.Errorf("no threshold public key for quorum hash %v", quorumHash)
}

// GetHeight ...
func (pv *FilePV) GetHeight(quorumHash crypto.QuorumHash) (int64, error) {
	intString := pv.Key.FirstHeightOfQuorums[quorumHash.String()]
	return strconv.ParseInt(intString,10, 64)
}

// ExtractIntoValidator ...
func (pv *FilePV) ExtractIntoValidator(quorumHash crypto.QuorumHash) *types.Validator {
	pubKey, _ := pv.GetPubKey(quorumHash)
	if len(pv.Key.ProTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &types.Validator{
		PubKey:      pubKey,
		VotingPower: types.DefaultDashVotingPower,
		ProTxHash:   pv.Key.ProTxHash,
	}
}

// GetProTxHash returns the pro tx hash of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetProTxHash() (crypto.ProTxHash, error) {
	if len(pv.Key.ProTxHash) != crypto.ProTxHashSize {
		return nil, fmt.Errorf("file proTxHash is invalid size")
	}
	return pv.Key.ProTxHash, nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	if err := pv.signVote(chainID, quorumType, quorumHash, vote); err != nil {
		return fmt.Errorf("error signing vote: %v", err)
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *FilePV) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) ([]byte, error) {
	signId, err := pv.signProposal(chainID, quorumType, quorumHash, proposal)
	if err != nil {
		return signId, fmt.Errorf("error signing proposal: %v", err)
	}
	return signId, nil
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() {
	pv.Key.Save()
	pv.LastSignState.Save()
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() {
	var blockSig []byte
	var stateSig []byte
	pv.LastSignState.Height = 0
	pv.LastSignState.Round = 0
	pv.LastSignState.Step = 0
	pv.LastSignState.BlockSignature = blockSig
	pv.LastSignState.StateSignature = stateSig
	pv.LastSignState.BlockSignBytes = nil
	pv.LastSignState.StateSignBytes = nil
	pv.Save()
}

// String returns a string representation of the FilePV.
func (pv *FilePV) String() string {
	return fmt.Sprintf(
		"PrivValidator{%v LH:%v, LR:%v, LS:%v}",
		pv.Key.ProTxHash,
		pv.LastSignState.Height,
		pv.LastSignState.Round,
		pv.LastSignState.Step,
	)
}

func (pv *FilePV) UpdatePrivateKey(privateKey crypto.PrivKey, quorumHash crypto.QuorumHash, height int64) error {
	pv.Key.PrivateKeys[quorumHash.String()] = crypto.QuorumKeys{
		PrivKey: privateKey,
		PubKey: privateKey.PubKey(),
	}
	pv.Key.UpdateHeights[strconv.Itoa(int(height))] = quorumHash
	if _, ok := pv.Key.FirstHeightOfQuorums[quorumHash.String()]; ok != true {
		pv.Key.FirstHeightOfQuorums[quorumHash.String()] = strconv.Itoa(int(height))
	}
	return nil
}

//------------------------------------------------------------------------------------

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	height, round, step := vote.Height, vote.Round, voteToStep(vote)

	lss := pv.LastSignState

	// The vote should not have a state ID set if the block ID is set to nil

	if vote.BlockID.Hash == nil && vote.StateID.LastAppHash != nil {
		return fmt.Errorf("error : vote should not have a state ID set if the"+
			" block ID for the round (%d/%d) is not set", vote.Height, vote.Round)
	}

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	blockSignId := types.VoteBlockSignId(chainID, vote, quorumType, quorumHash)

	stateSignId := types.VoteStateSignId(chainID, vote, quorumType, quorumHash)

	blockSignBytes := types.VoteBlockSignBytes(chainID, vote)

	stateSignBytes := types.VoteStateSignBytes(chainID, vote)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {

		if bytes.Equal(blockSignBytes, lss.BlockSignBytes) && bytes.Equal(stateSignBytes, lss.StateSignBytes) {
			vote.BlockSignature = lss.BlockSignature
			vote.StateSignature = lss.StateSignature
		} else {
			err = fmt.Errorf("conflicting data")
		}
		return err
	}

	var privKey crypto.PrivKey
	if quorumKeys, ok := pv.Key.PrivateKeys[quorumHash.String()]; ok {
		privKey = quorumKeys.PrivKey
	} else {
		return fmt.Errorf("file private validator could not sign vote for quorum hash %v", quorumHash)
	}

	sigBlock, err := privKey.SignDigest(blockSignId)
	if err != nil {
		return err
	}

	var sigState []byte
	if vote.BlockID.Hash != nil {
		sigState, err = privKey.SignDigest(stateSignId)
		if err != nil {
			return err
		}
	}

	//  if vote.BlockID.Hash == nil {
	//	  fmt.Printf("***********we are signing NIL (%d/%d) %X signed (file) for vote %v blockSignBytes %X\n",
	//	    vote.Height, vote.Round, sigBlock, vote, blockSignBytes)
	//  } else {
	//	  fmt.Printf("==block signature (%d/%d) %X signed (file) for vote %v\n", vote.Height, vote.Round,
	//	   sigBlock, vote)
	//  }

	pv.saveSigned(height, round, step, blockSignBytes, sigBlock, stateSignBytes, sigState)

	vote.BlockSignature = sigBlock
	vote.StateSignature = sigState

	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *FilePV) signProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) ([]byte, error) {
	height, round, step := proposal.Height, proposal.Round, stepPropose

	lss := pv.LastSignState

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return nil, err
	}

	blockSignId := types.ProposalBlockSignId(chainID, proposal, quorumType, quorumHash)

	blockSignBytes := types.ProposalBlockSignBytes(chainID, proposal)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	// If they only differ by timestamp, use last timestamp and signature
	// Otherwise, return error
	if sameHRS {
		if bytes.Equal(blockSignBytes, lss.BlockSignBytes) {
			proposal.Signature = lss.BlockSignBytes
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(lss.BlockSignBytes, blockSignBytes); ok {
			proposal.Timestamp = timestamp
			proposal.Signature = lss.BlockSignBytes
		} else {
			err = fmt.Errorf("conflicting data")
		}
		return blockSignId, err
	}

	var privKey crypto.PrivKey
	if quorumKeys, ok := pv.Key.PrivateKeys[quorumHash.String()]; ok {
		privKey = quorumKeys.PrivKey
	} else {
		return blockSignId, fmt.Errorf("file private validator could not sign vote for quorum hash %v", quorumHash)
	}

	// It passed the checks. SignDigest the proposal
	blockSig, err := privKey.SignDigest(blockSignId)
	if err != nil {
		return blockSignId, err
	}
	// fmt.Printf("file proposer %X \nsigning proposal at height %d \nwith key %X \nproposalSignId %X\n signature %X\n", pv.Key.ProTxHash,
	//  proposal.Height, pv.Key.PrivKey.PubKey().Bytes(), blockSignId, blockSig)

	pv.saveSigned(height, round, step, blockSignBytes, blockSig, nil, nil)
	proposal.Signature = blockSig
	return blockSignId, nil
}

// Persist height/round/step and signature
func (pv *FilePV) saveSigned(height int64, round int32, step int8,
	blockSignBytes []byte, blockSig []byte, stateSignBytes []byte, stateSig []byte) {

	pv.LastSignState.Height = height
	pv.LastSignState.Round = round
	pv.LastSignState.Step = step
	pv.LastSignState.BlockSignature = blockSig
	pv.LastSignState.BlockSignBytes = blockSignBytes
	pv.LastSignState.StateSignature = stateSig
	pv.LastSignState.StateSignBytes = stateSignBytes
	pv.LastSignState.Save()
}

//-----------------------------------------------------------------------------------------

// returns the timestamp from the lastSignBytes.
// returns true if the only difference in the proposals is their timestamp
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastProposal, newProposal tmproto.CanonicalProposal
	if err := protoio.UnmarshalDelimited(lastSignBytes, &lastProposal); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := protoio.UnmarshalDelimited(newSignBytes, &newProposal); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime := lastProposal.Timestamp
	// set the times to the same value and check equality
	now := tmtime.Now()
	lastProposal.Timestamp = now
	newProposal.Timestamp = now

	return lastTime, proto.Equal(&newProposal, &lastProposal)
}
