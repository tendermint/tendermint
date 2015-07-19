package state

import (
	"io/ioutil"
	"os"
	"time"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
	ptypes "github.com/tendermint/tendermint/permission/types"
	"github.com/tendermint/tendermint/types"
)

//------------------------------------------------------------
// we store the gendoc in the db

var GenDocKey = []byte("GenDocKey")

//------------------------------------------------------------
// core types for a genesis definition

type BasicAccount struct {
	Address []byte `json:"address"`
	Amount  int64  `json:"amount"`
}

type GenesisAccount struct {
	Address     []byte                     `json:"address"`
	Amount      int64                      `json:"amount"`
	Name        string                     `json:"name"`
	Permissions *ptypes.AccountPermissions `json:"permissions"`
}

type GenesisValidator struct {
	PubKey   acm.PubKeyEd25519 `json:"pub_key"`
	Amount   int64             `json:"amount"`
	Name     string            `json:"name"`
	UnbondTo []BasicAccount    `json:"unbond_to"`
}

type GenesisParams struct {
	GlobalPermissions *ptypes.AccountPermissions `json:"global_permissions"`
}

type GenesisDoc struct {
	GenesisTime time.Time          `json:"genesis_time"`
	ChainID     string             `json:"chain_id"`
	Params      *GenesisParams     `json:"params"`
	Accounts    []GenesisAccount   `json:"accounts"`
	Validators  []GenesisValidator `json:"validators"`
}

//------------------------------------------------------------
// Make genesis state from file

func GenesisDocFromJSON(jsonBlob []byte) (genState *GenesisDoc) {
	var err error
	binary.ReadJSONPtr(&genState, jsonBlob, &err)
	if err != nil {
		log.Error(Fmt("Couldn't read GenesisDoc: %v", err))
		os.Exit(1)
	}
	return
}

func MakeGenesisStateFromFile(db dbm.DB, genDocFile string) (*GenesisDoc, *State) {
	jsonBlob, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		log.Error(Fmt("Couldn't read GenesisDoc file: %v", err))
		os.Exit(1)
	}
	genDoc := GenesisDocFromJSON(jsonBlob)
	return genDoc, MakeGenesisState(db, genDoc)
}

func MakeGenesisState(db dbm.DB, genDoc *GenesisDoc) *State {
	if len(genDoc.Validators) == 0 {
		Exit(Fmt("The genesis file has no validators"))
	}

	if genDoc.GenesisTime.IsZero() {
		genDoc.GenesisTime = time.Now()
	}

	// Make accounts state tree
	accounts := merkle.NewIAVLTree(binary.BasicCodec, acm.AccountCodec, defaultAccountsCacheCapacity, db)
	for _, genAcc := range genDoc.Accounts {
		perm := ptypes.ZeroAccountPermissions
		if genAcc.Permissions != nil {
			perm = *genAcc.Permissions
		}
		acc := &acm.Account{
			Address:     genAcc.Address,
			PubKey:      nil,
			Sequence:    0,
			Balance:     genAcc.Amount,
			Permissions: perm,
		}
		accounts.Set(acc.Address, acc)
	}

	// global permissions are saved as the 0 address
	// so they are included in the accounts tree
	globalPerms := ptypes.DefaultAccountPermissions
	if genDoc.Params != nil && genDoc.Params.GlobalPermissions != nil {
		globalPerms = *genDoc.Params.GlobalPermissions
		// XXX: make sure the set bits are all true
		// Without it the HasPermission() functions will fail
		globalPerms.Base.SetBit = ptypes.AllPermFlags
	}

	permsAcc := &acm.Account{
		Address:     ptypes.GlobalPermissionsAddress,
		PubKey:      nil,
		Sequence:    0,
		Balance:     1337,
		Permissions: globalPerms,
	}
	accounts.Set(permsAcc.Address, permsAcc)

	// Make validatorInfos state tree && validators slice
	validatorInfos := merkle.NewIAVLTree(binary.BasicCodec, ValidatorInfoCodec, 0, db)
	validators := make([]*Validator, len(genDoc.Validators))
	for i, val := range genDoc.Validators {
		pubKey := val.PubKey
		address := pubKey.Address()

		// Make ValidatorInfo
		valInfo := &ValidatorInfo{
			Address:         address,
			PubKey:          pubKey,
			UnbondTo:        make([]*types.TxOutput, len(val.UnbondTo)),
			FirstBondHeight: 0,
			FirstBondAmount: val.Amount,
		}
		for i, unbondTo := range val.UnbondTo {
			valInfo.UnbondTo[i] = &types.TxOutput{
				Address: unbondTo.Address,
				Amount:  unbondTo.Amount,
			}
		}
		validatorInfos.Set(address, valInfo)

		// Make validator
		validators[i] = &Validator{
			Address:     address,
			PubKey:      pubKey,
			VotingPower: val.Amount,
		}
	}

	// Make namereg tree
	nameReg := merkle.NewIAVLTree(binary.BasicCodec, NameRegCodec, 0, db)
	// TODO: add names, contracts to genesis.json

	// IAVLTrees must be persisted before copy operations.
	accounts.Save()
	validatorInfos.Save()
	nameReg.Save()

	return &State{
		DB:                   db,
		ChainID:              genDoc.ChainID,
		LastBlockHeight:      0,
		LastBlockHash:        nil,
		LastBlockParts:       types.PartSetHeader{},
		LastBlockTime:        genDoc.GenesisTime,
		BondedValidators:     NewValidatorSet(validators),
		LastBondedValidators: NewValidatorSet(nil),
		UnbondingValidators:  NewValidatorSet(nil),
		accounts:             accounts,
		validatorInfos:       validatorInfos,
		nameReg:              nameReg,
	}
}
