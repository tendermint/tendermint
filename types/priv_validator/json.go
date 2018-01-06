package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

var _ types.PrivValidator = (*PrivValidatorJSON)(nil)

// PrivValidatorJSON wraps PrivValidatorUnencrypted
// and persists it to disk after every SignVote and SignProposal.
type PrivValidatorJSON struct {
	*PrivValidatorUnencrypted

	filePath string
}

// SignVote implements PrivValidator. It persists to disk.
func (pvj *PrivValidatorJSON) SignVote(chainID string, vote *types.Vote) error {
	err := pvj.PrivValidatorUnencrypted.SignVote(chainID, vote)
	if err != nil {
		return err
	}
	pvj.Save()
	return nil
}

// SignProposal implements PrivValidator. It persists to disk.
func (pvj *PrivValidatorJSON) SignProposal(chainID string, proposal *types.Proposal) error {
	err := pvj.PrivValidatorUnencrypted.SignProposal(chainID, proposal)
	if err != nil {
		return err
	}
	pvj.Save()
	return nil
}

//-------------------------------------------------------

// String returns a string representation of the PrivValidatorJSON.
func (pvj *PrivValidatorJSON) String() string {
	return fmt.Sprintf("PrivValidator{%v %v}", pvj.Address(), pvj.PrivValidatorUnencrypted.String())
}

func (pvj *PrivValidatorJSON) Save() {
	pvj.save()
}

func (pvj *PrivValidatorJSON) save() {
	if pvj.filePath == "" {
		cmn.PanicSanity("Cannot save PrivValidator: filePath not set")
	}
	jsonBytes, err := json.Marshal(pvj)
	if err != nil {
		// ; BOOM!!!
		cmn.PanicCrisis(err)
	}
	err = cmn.WriteFileAtomic(pvj.filePath, jsonBytes, 0600)
	if err != nil {
		// ; BOOM!!!
		cmn.PanicCrisis(err)
	}
}

// Reset resets the PrivValidatorUnencrypted. Panics if the Signer is the wrong type.
// NOTE: Unsafe!
func (pvj *PrivValidatorJSON) Reset() {
	pvj.PrivValidatorUnencrypted.LastSignedInfo.Reset()
	pvj.Save()
}

//----------------------------------------------------------------

// GenPrivValidatorJSON generates a new validator with randomly generated private key
// and the given filePath. It does not persist to file.
func GenPrivValidatorJSON(filePath string) *PrivValidatorJSON {
	privKey := crypto.GenPrivKeyEd25519().Wrap()
	return &PrivValidatorJSON{
		PrivValidatorUnencrypted: NewPrivValidatorUnencrypted(privKey),
		filePath:                 filePath,
	}
}

// LoadPrivValidatorJSON loads a PrivValidatorJSON from the filePath.
func LoadPrivValidatorJSON(filePath string) *PrivValidatorJSON {
	pvJSONBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		cmn.Exit(err.Error())
	}
	pvj := PrivValidatorJSON{}
	err = json.Unmarshal(pvJSONBytes, &pvj)
	if err != nil {
		cmn.Exit(cmn.Fmt("Error reading PrivValidatorJSON from %v: %v\n", filePath, err))
	}

	// enable persistence
	pvj.filePath = filePath
	return &pvj
}

// LoadOrGenPrivValidatorJSON loads a PrivValidatorJSON from the given filePath
// or else generates a new one and saves it to the filePath.
func LoadOrGenPrivValidatorJSON(filePath string) *PrivValidatorJSON {
	var pvj *PrivValidatorJSON
	if _, err := os.Stat(filePath); err == nil {
		pvj = LoadPrivValidatorJSON(filePath)
	} else {
		pvj = GenPrivValidatorJSON(filePath)
		pvj.Save()
	}
	return pvj
}

//--------------------------------------------------------------

// NewTestPrivValidator returns a PrivValidatorJSON with a tempfile
// for the file path.
func NewTestPrivValidator(signer types.TestSigner) *PrivValidatorJSON {
	_, tempFilePath := cmn.Tempfile("priv_validator_")
	pv := &PrivValidatorJSON{
		PrivValidatorUnencrypted: NewPrivValidatorUnencrypted(signer.(*types.DefaultTestSigner).PrivKey),
		filePath:                 tempFilePath,
	}
	return pv
}

//------------------------------------------------------

type PrivValidatorsByAddress []*PrivValidatorJSON

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
