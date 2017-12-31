package types

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

// DefaultPrivValidator implements PrivValidator.
type DefaultPrivValidator struct {
	ID            ValidatorID
	Signer        Signer
	CarefulSigner CarefulSigner
}

// Address returns the address of the validator.
// Implements PrivValidator.
func (pv *DefaultPrivValidator) Address() data.Bytes {
	return pv.ID.Address
}

// PubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *DefaultPrivValidator) PubKey() crypto.PubKey {
	return pv.ID.PubKey
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
}

//----------------------------------------------------------------

// GenDefaultPrivValidator generates a new validator with randomly generated private key
// and sets the filePath, but does not call Save().
func GenDefaultPrivValidator(filePath string, store PrivValidatorStore) *DefaultPrivValidator {
	privKey := crypto.GenPrivKeyEd25519().Wrap()
	id := ValidatorID{privKey.PubKey().Address(), privKey.PubKey()}

	info := PrivValidatorInfo{
		ID:   id,
		Type: TypePrivValidatorKeyStoreUnencrypted,
	}

	infoBytes, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}
	err = cmn.WriteFileAtomic(filePath, infoBytes, 0600)
	if err != nil {
		panic(err)
	}

	signer := NewDefaultSigner(privKey)
	carefulSigner := NewLastSignedInfo(store.SetCarefulSigner)

	return &DefaultPrivValidator{
		ID:            id,
		Signer:        signer,
		CarefulSigner: carefulSigner,
	}
}

// LoadDefaultPrivValidator loads a DefaultPrivValidator from the filePath.
func LoadDefaultPrivValidator(filePath string, store PrivValidatorStore) *DefaultPrivValidator {
	pviJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	pvi := PrivValidatorInfo{}
	err = json.Unmarshal(pviJSONBytes, &pvi)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading PrivValidatorInfo from %v: %v\n", filePath, err))
	}

	return &DefaultPrivValidator{
		ID:            pvi.ID,
		Signer:        store.GetSigner(pvi.Type),
		CarefulSigner: store.GetCarefulSigner(pvi.Type),
	}
}

//------------------------

// LoadOrGenDefaultPrivValidator loads a DefaultPrivValidator from the given filePath
// or else generates a new one and saves it to the filePath.
func LoadOrGenDefaultPrivValidator(filePath string, store PrivValidatorStore) *DefaultPrivValidator {
	var pv *DefaultPrivValidator
	if _, err := os.Stat(filePath); err == nil {
		pv = LoadDefaultPrivValidator(filePath, store)
	} else {
		pv = GenDefaultPrivValidator(filePath, store)
	}
	return pv
}

//------------------------

/*
// LoadPrivValidatorWithSigner loads a DefaultPrivValidator with a custom
// signer object. The DefaultPrivValidator handles double signing prevention by persisting
// data to the filePath, while the Signer handles the signing.
// If the filePath does not exist, the DefaultPrivValidator must be created manually and saved.
func LoadDefaultPrivValidatorWithSigner(filePath string, signerFunc func(PrivValidator) Signer) *DefaultPrivValidator {
	pvJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	pv := &DefaultPrivValidator{}
	err = json.Unmarshal(pvJSONBytes, &pv)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading PrivValidator from %v: %v\n", filePath, err))
	}

	pv.filePath = filePath
	pv.Signer = signerFunc(pv)
	return pv
}

// Save persists the DefaultPrivValidator to disk.
func (pv *DefaultPrivValidator) Save() {
	pv.mtx.Lock()
	defer pv.mtx.Unlock()
	pv.save()
}

func (pv *DefaultPrivValidator) save() {
	if pv.filePath == "" {
		cmn.PanicSanity("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes, err := json.Marshal(pv)
	if err != nil {
		// `@; BOOM!!!
		cmn.PanicCrisis(err)
	}
	err = cmn.WriteFileAtomic(pv.filePath, jsonBytes, 0600)
	if err != nil {
		// `@; BOOM!!!
		cmn.PanicCrisis(err)
	}
}

// UnmarshalJSON unmarshals the given jsonString
// into a DefaultPrivValidator using a DefaultSigner.
func (pv *DefaultPrivValidator) UnmarshalJSON(jsonString []byte) error {
	idAndInfo := &struct {
		ID   ValidatorID    `json:"id"`
		Info LastSignedInfo `json:"info"`
	}{}
	if err := json.Unmarshal(jsonString, idAndInfo); err != nil {
		return err
	}

	signer := &struct {
		Signer *DefaultSigner `json:"signer"`
	}{}
	if err := json.Unmarshal(jsonString, signer); err != nil {
		return err
	}
	fmt.Println("STRING", string(jsonString))
	fmt.Println("SIGNER", signer)

	pv.ID = idAndInfo.ID
	pv.Info = idAndInfo.Info
	pv.Signer = signer.Signer
	return nil
}

// Reset resets all fields in the DefaultPrivValidator.
// NOTE: Unsafe!
func (pv *DefaultPrivValidator) Reset() {
	pv.Info.Reset()
	pv.Save()
}

*/

//-------------------------------------
