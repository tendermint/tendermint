package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

var _ PrivValidator = (*DefaultPrivValidator)(nil)

// DefaultPrivValidator implements PrivValidator.
type DefaultPrivValidator struct {
	Info          PrivValidatorInfo     `json:"info"`
	Signer        *DefaultSigner        `json:"signer"`
	CarefulSigner *DefaultCarefulSigner `json:"careful_signer"`

	filePath string
}

func NewTestPrivValidator(signer types.TestSigner) *DefaultPrivValidator {
	_, tempFilePath := cmn.Tempfile("priv_validator_")
	pv := &DefaultPrivValidator{
		Info: PrivValidatorInfo{
			ID: ValidatorID{
				Address: signer.Address(),
				PubKey:  signer.PubKey(),
			},
		},
		Signer:   NewDefaultSigner(signer.(*types.DefaultTestSigner).PrivKey),
		filePath: tempFilePath,
	}
	pv.CarefulSigner = NewDefaultCarefulSigner(pv.defaultSaveFn)
	return pv
}

// Address returns the address of the validator.
// Implements PrivValidator.
func (pv *DefaultPrivValidator) Address() data.Bytes {
	return pv.Info.ID.Address
}

// PubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *DefaultPrivValidator) PubKey() crypto.PubKey {
	return pv.Info.ID.PubKey
}

// SignVote signs a canonical representation of the vote, along with the
// chainID. Implements PrivValidator.
func (pv *DefaultPrivValidator) SignVote(chainID string, vote *types.Vote) error {
	return pv.CarefulSigner.SignVote(pv.Signer, chainID, vote)
}

// SignProposal signs a canonical representation of the proposal, along with
// the chainID. Implements PrivValidator.
func (pv *DefaultPrivValidator) SignProposal(chainID string, proposal *types.Proposal) error {
	return pv.CarefulSigner.SignProposal(pv.Signer, chainID, proposal)
}

// SignHeartbeat signs a canonical representation of the heartbeat, along with the chainID.
// Implements PrivValidator.
func (pv *DefaultPrivValidator) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) error {
	return pv.CarefulSigner.SignHeartbeat(pv.Signer, chainID, heartbeat)
}

// String returns a string representation of the DefaultPrivValidator.
func (pv *DefaultPrivValidator) String() string {
	return fmt.Sprintf("PrivValidator{%v %v}", pv.Address(), pv.CarefulSigner.String())
}

func (pv *DefaultPrivValidator) Save() {
	pv.save()
}

func (pv *DefaultPrivValidator) save() {
	if pv.filePath == "" {
		cmn.PanicSanity("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes, err := json.Marshal(pv)
	if err != nil {
		// ; BOOM!!!
		cmn.PanicCrisis(err)
	}
	err = cmn.WriteFileAtomic(pv.filePath, jsonBytes, 0600)
	if err != nil {
		// ; BOOM!!!
		cmn.PanicCrisis(err)
	}
}

func (pv *DefaultPrivValidator) defaultSaveFn(lsi LastSignedInfo) {
	pv.Save()
}

// Reset resets all fields in the DefaultPrivValidator.
// NOTE: Unsafe!
func (pv *DefaultPrivValidator) Reset() {
	pv.CarefulSigner.Reset()
	pv.Save()
}

//----------------------------------------------------------------

// GenDefaultPrivValidator generates a new validator with randomly generated private key.
func GenDefaultPrivValidator(filePath string) *DefaultPrivValidator {
	privKey := crypto.GenPrivKeyEd25519().Wrap()
	id := ValidatorID{privKey.PubKey().Address(), privKey.PubKey()}

	info := PrivValidatorInfo{
		ID:   id,
		Type: TypePrivValidatorKeyStoreUnencrypted,
	}

	signer := NewDefaultSigner(privKey)

	pv := &DefaultPrivValidator{
		Info:     info,
		Signer:   signer,
		filePath: filePath,
	}
	pv.CarefulSigner = NewDefaultCarefulSigner(pv.defaultSaveFn)
	return pv
}

// LoadDefaultPrivValidator loads a DefaultPrivValidator from the filePath.
func LoadDefaultPrivValidator(filePath string) *DefaultPrivValidator {
	pvJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	pv := DefaultPrivValidator{}
	err = json.Unmarshal(pvJSONBytes, &pv)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading DefaultPrivValidator from %v: %v\n", filePath, err))
	}

	// enable persistence
	pv.filePath = filePath
	pv.CarefulSigner.saveFn = pv.defaultSaveFn

	return &pv
}

// LoadOrGenDefaultPrivValidator loads a DefaultPrivValidator from the given filePath
// or else generates a new one and saves it to the filePath.
func LoadOrGenDefaultPrivValidator(filePath string) *DefaultPrivValidator {
	var pv *DefaultPrivValidator
	if _, err := os.Stat(filePath); err == nil {
		pv = LoadDefaultPrivValidator(filePath)
	} else {
		pv = GenDefaultPrivValidator(filePath)
		pv.Save()
	}
	return pv
}

//--------------------------------------------------------------

type PrivValidatorsByAddress []*DefaultPrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].Address(), pvs[j].Address()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}
