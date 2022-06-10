package privval

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/internal/libs/tempfile"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// TODO: type ?
const (
	stepNone      int8 = 0 // Used to distinguish the initial state
	stepPropose   int8 = 1
	stepPrevote   int8 = 2
	stepPrecommit int8 = 3
)

// A vote is either stepPrevote or stepPrecommit.
func voteToStep(vote *tmproto.Vote) (int8, error) {
	switch vote.Type {
	case tmproto.PrevoteType:
		return stepPrevote, nil
	case tmproto.PrecommitType:
		return stepPrecommit, nil
	default:
		return 0, fmt.Errorf("unknown vote type: %v", vote.Type)
	}
}

//-------------------------------------------------------------------------------

// FilePVKey stores the immutable part of PrivValidator.
type FilePVKey struct {
	PrivateKeys map[string]crypto.QuorumKeys
	// heightString -> quorumHash
	UpdateHeights map[string]crypto.QuorumHash
	// quorumHash -> heightString
	FirstHeightOfQuorums map[string]string
	ProTxHash            crypto.ProTxHash

	filePath string
}

type filePVKeyJSON struct {
	PrivateKeys map[string]crypto.QuorumKeys `json:"private_keys"`
	// heightString -> quorumHash
	UpdateHeights map[string]crypto.QuorumHash `json:"update_heights"`
	// quorumHash -> heightString
	FirstHeightOfQuorums map[string]string `json:"first_height_of_quorums"`
	ProTxHash            crypto.ProTxHash  `json:"pro_tx_hash"`
}

func (pvKey FilePVKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(filePVKeyJSON{
		PrivateKeys:          pvKey.PrivateKeys,
		UpdateHeights:        pvKey.UpdateHeights,
		FirstHeightOfQuorums: pvKey.FirstHeightOfQuorums,
		ProTxHash:            pvKey.ProTxHash,
	})
}

func (pvKey *FilePVKey) UnmarshalJSON(data []byte) error {
	var key filePVKeyJSON
	if err := json.Unmarshal(data, &key); err != nil {
		return err
	}
	pvKey.PrivateKeys = key.PrivateKeys
	pvKey.UpdateHeights = key.UpdateHeights
	pvKey.FirstHeightOfQuorums = key.FirstHeightOfQuorums
	pvKey.ProTxHash = key.ProTxHash
	return nil
}

// Save persists the FilePVKey to its filePath.
func (pvKey FilePVKey) Save() error {
	outFile := pvKey.filePath
	if outFile == "" {
		return errors.New("cannot save PrivValidator key: filePath not set")
	}

	data, err := json.MarshalIndent(pvKey, "", "  ")
	if err != nil {
		return err
	}
	return tempfile.WriteFileAtomic(outFile, data, 0600)
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
	Height         int64            `json:"height,string"`
	Round          int32            `json:"round"`
	Step           int8             `json:"step"`
	BlockSignature []byte           `json:"block_signature,omitempty"`
	BlockSignBytes tmbytes.HexBytes `json:"block_sign_bytes,omitempty"`
	StateSignature []byte           `json:"state_signature,omitempty"`
	StateSignBytes tmbytes.HexBytes `json:"state_sign_bytes,omitempty"`

	filePath string
}

func (lss *FilePVLastSignState) reset() {
	lss.Height = 0
	lss.Round = 0
	lss.Step = 0
	lss.BlockSignature = nil
	lss.BlockSignBytes = nil
	lss.StateSignature = nil
	lss.StateSignBytes = nil
}

// checkHRS checks the given height, round, step (HRS) against that of the
// FilePVLastSignState. It returns an error if the arguments constitute a regression,
// or if they match but the SignBytes are empty.
// The returned boolean indicates whether the last Signature should be reused -
// it returns true if the HRS matches the arguments and the SignBytes are not empty (indicating
// we have already signed for this HRS, and can reuse the existing signature).
// It panics if the HRS matches the arguments, there's a SignBytes, but no Signature.
func (lss *FilePVLastSignState) checkHRS(height int64, round int32, step int8) (bool, error) {

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
func (lss *FilePVLastSignState) Save() error {
	outFile := lss.filePath
	if outFile == "" {
		return errors.New("cannot save FilePVLastSignState: filePath not set")
	}
	jsonBytes, err := json.MarshalIndent(lss, "", "  ")
	if err != nil {
		return err
	}
	return tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
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
	mtx           sync.RWMutex
}

// FilePVOption ...
type FilePVOption func(filePV *FilePV) error

// NewFilePVOneKey generates a new validator from the given key and paths.
func NewFilePVOneKey(
	privKey crypto.PrivKey, proTxHash []byte, quorumHash crypto.QuorumHash,
	thresholdPublicKey crypto.PubKey, keyFilePath, stateFilePath string,
) *FilePV {
	if len(proTxHash) != crypto.ProTxHashSize {
		panic("error setting incorrect proTxHash size in NewFilePV")
	}

	if thresholdPublicKey == nil {
		thresholdPublicKey = privKey.PubKey()
	}

	quorumKeys := crypto.QuorumKeys{
		PrivKey:            privKey,
		PubKey:             privKey.PubKey(),
		ThresholdPublicKey: thresholdPublicKey,
	}
	privateKeysMap := make(map[string]crypto.QuorumKeys)
	privateKeysMap[quorumHash.String()] = quorumKeys

	updateHeightsMap := make(map[string]crypto.QuorumHash)
	firstHeightOfQuorumsMap := make(map[string]string)

	return &FilePV{
		Key: FilePVKey{
			PrivateKeys:          privateKeysMap,
			ProTxHash:            proTxHash,
			filePath:             keyFilePath,
			UpdateHeights:        updateHeightsMap,
			FirstHeightOfQuorums: firstHeightOfQuorumsMap,
		},
		LastSignState: FilePVLastSignState{
			Step:     stepNone,
			filePath: stateFilePath,
		},
	}
}

var _ types.PrivValidator = (*FilePV)(nil)

// WithKeyAndStateFilePaths ...
func WithKeyAndStateFilePaths(keyFilePath, stateFilePath string) FilePVOption {
	return func(filePV *FilePV) error {
		filePV.Key.filePath = keyFilePath
		filePV.LastSignState.filePath = stateFilePath
		return nil
	}
}

func WithPrivateKey(key crypto.PrivKey, quorumHash crypto.QuorumHash, thresholdPublicKey *crypto.PubKey) FilePVOption {
	if thresholdPublicKey == nil {
		return WithPrivateKeys([]crypto.PrivKey{key}, []crypto.QuorumHash{quorumHash}, nil)
	}
	return WithPrivateKeys([]crypto.PrivKey{key}, []crypto.QuorumHash{quorumHash}, &[]crypto.PubKey{*thresholdPublicKey})
}

// WithPrivateKeys ...
func WithPrivateKeys(
	keys []crypto.PrivKey, quorumHashes []crypto.QuorumHash, thresholdPublicKeys *[]crypto.PubKey,
) FilePVOption {
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
				PubKey:  key.PubKey(),
			}
			if thresholdPublicKeys != nil {
				quorumKeys.ThresholdPublicKey = (*thresholdPublicKeys)[i]
			}
			privateKeysMap[quorumHashes[i].String()] = quorumKeys
		}
		filePV.Key.PrivateKeys = privateKeysMap
		return nil
	}
}

func WithPrivateKeysMap(privateKeysMap map[string]crypto.QuorumKeys) FilePVOption {
	return func(filePV *FilePV) error {

		filePV.Key.PrivateKeys = privateKeysMap
		return nil
	}
}

func WithUpdateHeights(updateHeights map[string]crypto.QuorumHash) FilePVOption {
	return func(filePV *FilePV) error {
		filePV.Key.UpdateHeights = updateHeights
		if filePV.Key.FirstHeightOfQuorums == nil {
			filePV.Key.FirstHeightOfQuorums = make(map[string]string)
		}
		for height, quorumHash := range updateHeights {
			if _, ok := filePV.Key.FirstHeightOfQuorums[quorumHash.String()]; !ok {
				filePV.Key.FirstHeightOfQuorums[quorumHash.String()] = height
			}
		}
		return nil
	}
}

// WithProTxHash ...
func WithProTxHash(proTxHash types.ProTxHash) FilePVOption {
	return func(filePV *FilePV) error {
		if len(proTxHash) != crypto.ProTxHashSize {
			return fmt.Errorf("error setting incorrect proTxHash size in NewFilePV")
		}
		filePV.Key.ProTxHash = proTxHash.Copy()
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
func LoadFilePV(keyFilePath, stateFilePath string) (*FilePV, error) {
	return loadFilePV(keyFilePath, stateFilePath, true)
}

// LoadFilePVEmptyState loads a FilePV from the given keyFilePath, with an empty LastSignState.
// If the keyFilePath does not exist, the program will exit.
func LoadFilePVEmptyState(keyFilePath, stateFilePath string) (*FilePV, error) {
	return loadFilePV(keyFilePath, stateFilePath, false)
}

// If loadState is true, we load from the stateFilePath. Otherwise, we use an empty LastSignState.
func loadFilePV(keyFilePath, stateFilePath string, loadState bool) (*FilePV, error) {
	keyJSONBytes, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	pvKey := FilePVKey{}
	err = json.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		return nil, fmt.Errorf("error reading PrivValidator key from %v: %w", keyFilePath, err)
	}
	// verify proTxHash is 32 bytes if it exists
	if pvKey.ProTxHash != nil && len(pvKey.ProTxHash) != crypto.ProTxHashSize {
		return nil, fmt.Errorf("loadFilePV proTxHash must be 32 bytes in key file path %s", keyFilePath)
	}

	pvKey.filePath = keyFilePath

	pvState := FilePVLastSignState{}

	if loadState {
		stateJSONBytes, err := os.ReadFile(stateFilePath)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(stateJSONBytes, &pvState)
		if err != nil {
			return nil, fmt.Errorf("error reading PrivValidator state from %v: %w", stateFilePath, err)
		}
	}

	pvState.filePath = stateFilePath

	return &FilePV{
		Key:           pvKey,
		LastSignState: pvState,
	}, nil
}

// LoadOrGenFilePV loads a FilePV from the given filePaths
// or else generates a new one and saves it to the filePaths.
func LoadOrGenFilePV(keyFilePath, stateFilePath string) (*FilePV, error) {
	if tmos.FileExists(keyFilePath) {
		pv, err := LoadFilePV(keyFilePath, stateFilePath)
		if err != nil {
			return nil, err
		}
		return pv, nil
	}
	pv := GenFilePV(keyFilePath, stateFilePath)
	if err := pv.Save(); err != nil {
		return nil, err
	}

	return pv, nil
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey(context context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	if keys, ok := pv.Key.PrivateKeys[quorumHash.String()]; ok {
		return keys.PubKey, nil
	}
	return nil, fmt.Errorf("filePV: no public key for quorum hash (get) %v", quorumHash)
}

// GetFirstPubKey returns the first public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetFirstPubKey(context context.Context) (crypto.PubKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	for _, quorumKeys := range pv.Key.PrivateKeys {
		return quorumKeys.PubKey, nil
	}
	return nil, nil
}

func (pv *FilePV) GetQuorumHashes(context context.Context) ([]crypto.QuorumHash, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	quorumHashes := make([]crypto.QuorumHash, len(pv.Key.PrivateKeys))
	i := 0
	for quorumHashString := range pv.Key.PrivateKeys {
		quorumHash, err := hex.DecodeString(quorumHashString)
		quorumHashes[i] = quorumHash
		if err != nil {
			return nil, err
		}
		i++
	}
	return quorumHashes, nil
}

func (pv *FilePV) GetFirstQuorumHash(context context.Context) (crypto.QuorumHash, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	for quorumHashString := range pv.Key.PrivateKeys {
		return hex.DecodeString(quorumHashString)
	}
	return nil, nil
}

// GetThresholdPublicKey ...
func (pv *FilePV) GetThresholdPublicKey(context context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	if keys, ok := pv.Key.PrivateKeys[quorumHash.String()]; ok {
		return keys.ThresholdPublicKey, nil
	}
	return nil, fmt.Errorf("no threshold public key for quorum hash %v", quorumHash)
}

// GetPrivateKey ...
func (pv *FilePV) GetPrivateKey(context context.Context, quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	return pv.getPrivateKey(context, quorumHash)
}

func (pv *FilePV) getPrivateKey(context context.Context, quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	if keys, ok := pv.Key.PrivateKeys[quorumHash.String()]; ok {
		return keys.PrivKey, nil
	}
	hashes := make([]string, 0, len(pv.Key.PrivateKeys))
	for hash := range pv.Key.PrivateKeys {
		hashes = append(hashes, hash[:8])
	}
	return nil, fmt.Errorf("no private key for quorum hash %v, supported are: %v", quorumHash, hashes)
}

func (pv *FilePV) GetPublicKey(context context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	privateKey, err := pv.getPrivateKey(context, quorumHash)
	if err != nil {
		return nil, fmt.Errorf("no public key for quorum hash %v", quorumHash)
	}

	return privateKey.PubKey(), nil
}

// GetHeight ...
func (pv *FilePV) GetHeight(_ context.Context, quorumHash crypto.QuorumHash) (int64, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	if intString, ok := pv.Key.FirstHeightOfQuorums[quorumHash.String()]; ok {
		return strconv.ParseInt(intString, 10, 64)
	}
	return -1, fmt.Errorf("quorum hash not found for GetHeight %v", quorumHash.String())
}

// ExtractIntoValidator ...
func (pv *FilePV) ExtractIntoValidator(ctx context.Context, quorumHash crypto.QuorumHash) *types.Validator {
	pubKey, _ := pv.GetPubKey(ctx, quorumHash)
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

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
func (pv *FilePV) GetProTxHash(context context.Context) (crypto.ProTxHash, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	if len(pv.Key.ProTxHash) != crypto.ProTxHashSize {
		return nil, fmt.Errorf("file proTxHash is invalid size")
	}
	return pv.Key.ProTxHash, nil
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *FilePV) SignVote(
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	vote *tmproto.Vote,
	stateID types.StateID,
	logger log.Logger,
) error {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	if err := pv.signVote(ctx, chainID, quorumType, quorumHash, vote, stateID); err != nil {
		return fmt.Errorf("error signing vote: %v", err)
	}
	return nil
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *FilePV) SignProposal(
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	proposal *tmproto.Proposal,
) (tmbytes.HexBytes, error) {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	signID, err := pv.signProposal(ctx, chainID, quorumType, quorumHash, proposal)
	if err != nil {
		return signID, fmt.Errorf("error signing proposal: %v", err)
	}
	return signID, nil
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() error {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()

	if err := pv.Key.Save(); err != nil {
		return err
	}
	return pv.LastSignState.Save()
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() error {
	pv.mtx.Lock()
	pv.LastSignState.reset()
	pv.mtx.Unlock()
	return pv.Save()
}

// String returns a string representation of the FilePV.
func (pv *FilePV) String() string {
	pv.mtx.RLock()
	defer pv.mtx.RUnlock()

	return fmt.Sprintf(
		"PrivValidator{%v LH:%v, LR:%v, LS:%v}",
		pv.Key.ProTxHash,
		pv.LastSignState.Height,
		pv.LastSignState.Round,
		pv.LastSignState.Step,
	)
}

func (pv *FilePV) UpdatePrivateKey(
	context context.Context,
	privateKey crypto.PrivKey,
	quorumHash crypto.QuorumHash,
	thresholdPublicKey crypto.PubKey,
	height int64,
) {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()

	pv.Key.PrivateKeys[quorumHash.String()] = crypto.QuorumKeys{
		PrivKey:            privateKey,
		PubKey:             privateKey.PubKey(),
		ThresholdPublicKey: thresholdPublicKey,
	}
	pv.Key.UpdateHeights[strconv.Itoa(int(height))] = quorumHash
	if _, ok := pv.Key.FirstHeightOfQuorums[quorumHash.String()]; !ok {
		pv.Key.FirstHeightOfQuorums[quorumHash.String()] = strconv.Itoa(int(height))
	}
}

//------------------------------------------------------------------------------------

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	vote *tmproto.Vote,
	stateID types.StateID,
) error {
	step, err := voteToStep(vote)
	if err != nil {
		return err
	}

	height := vote.Height
	round := vote.Round
	lss := pv.LastSignState

	sameHRS, err := lss.checkHRS(height, round, step)
	if err != nil {
		return err
	}

	// StateID should refer to previous height in order to be valid
	if stateID.Height != height-1 {
		return fmt.Errorf("invalid height in StateID: is %d, should be %d", stateID.Height, height-1)
	}

	blockSignID := types.VoteBlockSignID(chainID, vote, quorumType, quorumHash)

	stateSignID := stateID.SignID(chainID, quorumType, quorumHash)

	blockSignBytes := types.VoteBlockSignBytes(chainID, vote)

	stateSignBytes := stateID.SignBytes(chainID)

	privKey, err := pv.getPrivateKey(ctx, quorumHash)
	if err != nil {
		return err
	}

	// Vote extensions are non-deterministic, so it is possible that an
	// application may have created a different extension. We therefore always
	// re-sign the vote extensions of precommits. For prevotes, the extension
	// signature will always be empty.
	var extSig []byte
	if vote.Type == tmproto.PrecommitType {
		extSignID := types.VoteExtensionSignID(chainID, vote, quorumType, quorumHash)
		extSig, err = privKey.SignDigest(extSignID)
		if err != nil {
			return err
		}
	} else if len(vote.Extension) > 0 {
		return errors.New("unexpected vote extension - extensions are only allowed in precommits")
	}

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
			return errors.New("conflicting data")
		}
		vote.ExtensionSignature = extSig
		return nil
	}

	sigBlock, err := privKey.SignDigest(blockSignID)
	if err != nil {
		return err
	}

	var sigState []byte
	if vote.BlockID.Hash != nil {
		sigState, err = privKey.SignDigest(stateSignID)
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

	err = pv.saveSigned(height, round, step, blockSignBytes, sigBlock, stateSignBytes, sigState)
	if err != nil {
		return err
	}

	vote.BlockSignature = sigBlock
	vote.StateSignature = sigState
	vote.ExtensionSignature = extSig

	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *FilePV) signProposal(
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	proposal *tmproto.Proposal,
) ([]byte, error) {
	height, round, step := proposal.Height, proposal.Round, stepPropose

	lss := pv.LastSignState

	sameHRS, err := lss.checkHRS(height, round, step)
	if err != nil {
		return nil, err
	}

	blockSignID := types.ProposalBlockSignID(chainID, proposal, quorumType, quorumHash)

	blockSignBytes := types.ProposalBlockSignBytes(chainID, proposal)

	// We might crash before writing to the wal,
	// causing us to try to re-sign for the same HRS.
	// If signbytes are the same, use the last signature.
	if sameHRS {
		if !bytes.Equal(blockSignBytes, lss.BlockSignBytes) {
			return nil, errors.New("conflicting data")
		}
		proposal.Signature = lss.BlockSignBytes
		return blockSignID, err
	}

	privKey, err := pv.getPrivateKey(ctx, quorumHash)
	if err != nil {
		return blockSignID, err
	}

	// It passed the checks. SignDigest the proposal
	blockSig, err := privKey.SignDigest(blockSignID)
	if err != nil {
		return blockSignID, err
	}
	// fmt.Printf(
	// "file proposer %X \nsigning proposal at height %d \nwith key %X \nproposalSignId %X\n signature %X\n",
	// pv.Key.ProTxHash,
	// proposal.Height, pv.Key.PrivKey.PubKey().Bytes(), blockSignID, blockSig)

	err = pv.saveSigned(height, round, step, blockSignBytes, blockSig, nil, nil)
	if err != nil {
		return nil, err
	}
	proposal.Signature = blockSig
	return blockSignID, nil
}

// Persist height/round/step and signature
func (pv *FilePV) saveSigned(
	height int64,
	round int32,
	step int8,
	blockSignBytes []byte,
	blockSig []byte,
	stateSignBytes []byte,
	stateSig []byte,
) error {
	pv.LastSignState.Height = height
	pv.LastSignState.Round = round
	pv.LastSignState.Step = step
	pv.LastSignState.BlockSignature = blockSig
	pv.LastSignState.BlockSignBytes = blockSignBytes
	pv.LastSignState.StateSignature = stateSig
	pv.LastSignState.StateSignBytes = stateSignBytes
	return pv.LastSignState.Save()
}
