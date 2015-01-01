package state

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"time"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

type GenesisAccount struct {
	Address string
	Amount  uint64
}

type GenesisValidator struct {
	PubKey   string
	Amount   uint64
	UnbondTo []GenesisAccount
}

type GenesisDoc struct {
	GenesisTime time.Time
	Accounts    []GenesisAccount
	Validators  []GenesisValidator
}

func GenesisDocFromJSON(jsonBlob []byte) (genState *GenesisDoc) {
	err := json.Unmarshal(jsonBlob, &genState)
	if err != nil {
		panic(Fmt("Couldn't read GenesisDoc: %v", err))
	}
	return
}

func MakeGenesisStateFromFile(db db_.DB, genDocFile string) *State {
	jsonBlob, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		panic(Fmt("Couldn't read GenesisDoc file: %v", err))
	}
	genDoc := GenesisDocFromJSON(jsonBlob)
	return MakeGenesisState(db, genDoc)
}

func MakeGenesisState(db db_.DB, genDoc *GenesisDoc) *State {
	if len(genDoc.Validators) == 0 {
		Exit(Fmt("The genesis file has no validators"))
	}

	if genDoc.GenesisTime.IsZero() {
		genDoc.GenesisTime = time.Now()
	}

	// Make accounts state tree
	accounts := merkle.NewIAVLTree(BasicCodec, AccountCodec, defaultAccountsCacheCapacity, db)
	for _, acc := range genDoc.Accounts {
		address, err := hex.DecodeString(acc.Address)
		if err != nil {
			Exit(Fmt("Invalid account address: %v", acc.Address))
		}
		account := &Account{
			Address:  address,
			PubKey:   PubKeyNil{},
			Sequence: 0,
			Balance:  acc.Amount,
		}
		accounts.Set(address, account)
	}

	// Make validatorInfos state tree && validators slice
	validatorInfos := merkle.NewIAVLTree(BasicCodec, ValidatorInfoCodec, 0, db)
	validators := make([]*Validator, len(genDoc.Validators))
	for i, val := range genDoc.Validators {
		pubKeyBytes, err := hex.DecodeString(val.PubKey)
		if err != nil {
			Exit(Fmt("Invalid validator pubkey: %v", val.PubKey))
		}
		pubKey := ReadBinary(PubKeyEd25519{},
			bytes.NewBuffer(pubKeyBytes), new(int64), &err).(PubKeyEd25519)
		if err != nil {
			Exit(Fmt("Invalid validator pubkey: %v", val.PubKey))
		}
		address := pubKey.Address()

		// Make ValidatorInfo
		valInfo := &ValidatorInfo{
			Address:         address,
			PubKey:          pubKey,
			UnbondTo:        make([]*TxOutput, len(val.UnbondTo)),
			FirstBondHeight: 0,
			FirstBondAmount: val.Amount,
		}
		for i, unbondTo := range val.UnbondTo {
			address, err := hex.DecodeString(unbondTo.Address)
			if err != nil {
				Exit(Fmt("Invalid unbond-to address: %v", unbondTo.Address))
			}
			valInfo.UnbondTo[i] = &TxOutput{
				Address: address,
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

	return &State{
		DB:                  db,
		LastBlockHeight:     0,
		LastBlockHash:       nil,
		LastBlockParts:      PartSetHeader{},
		LastBlockTime:       genDoc.GenesisTime,
		BondedValidators:    NewValidatorSet(validators),
		UnbondingValidators: NewValidatorSet(nil),
		accounts:            accounts,
		validatorInfos:      validatorInfos,
	}
}
