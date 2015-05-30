package state

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/types"
)

var (
	stateKey                     = []byte("stateKey")
	minBondAmount                = uint64(1)           // TODO adjust
	defaultAccountsCacheCapacity = 1000                // TODO adjust
	unbondingPeriodBlocks        = uint(60 * 24 * 365) // TODO probably better to make it time based.
	validatorTimeoutBlocks       = uint(10)            // TODO adjust
)

//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	DB                   dbm.DB
	ChainID              string
	LastBlockHeight      uint
	LastBlockHash        []byte
	LastBlockParts       types.PartSetHeader
	LastBlockTime        time.Time
	BondedValidators     *ValidatorSet
	LastBondedValidators *ValidatorSet
	UnbondingValidators  *ValidatorSet
	accounts             merkle.Tree // Shouldn't be accessed directly.
	validatorInfos       merkle.Tree // Shouldn't be accessed directly.

	evc events.Fireable // typically an events.EventCache
}

func LoadState(db dbm.DB) *State {
	s := &State{DB: db}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int64), new(error)
		s.ChainID = binary.ReadString(r, n, err)
		s.LastBlockHeight = binary.ReadUvarint(r, n, err)
		s.LastBlockHash = binary.ReadByteSlice(r, n, err)
		s.LastBlockParts = binary.ReadBinary(types.PartSetHeader{}, r, n, err).(types.PartSetHeader)
		s.LastBlockTime = binary.ReadTime(r, n, err)
		s.BondedValidators = binary.ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		s.LastBondedValidators = binary.ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		s.UnbondingValidators = binary.ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		accountsHash := binary.ReadByteSlice(r, n, err)
		s.accounts = merkle.NewIAVLTree(binary.BasicCodec, account.AccountCodec, defaultAccountsCacheCapacity, db)
		s.accounts.Load(accountsHash)
		validatorInfosHash := binary.ReadByteSlice(r, n, err)
		s.validatorInfos = merkle.NewIAVLTree(binary.BasicCodec, ValidatorInfoCodec, 0, db)
		s.validatorInfos.Load(validatorInfosHash)
		if *err != nil {
			panic(*err)
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}

func (s *State) Save() {
	s.accounts.Save()
	s.validatorInfos.Save()
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	binary.WriteString(s.ChainID, buf, n, err)
	binary.WriteUvarint(s.LastBlockHeight, buf, n, err)
	binary.WriteByteSlice(s.LastBlockHash, buf, n, err)
	binary.WriteBinary(s.LastBlockParts, buf, n, err)
	binary.WriteTime(s.LastBlockTime, buf, n, err)
	binary.WriteBinary(s.BondedValidators, buf, n, err)
	binary.WriteBinary(s.LastBondedValidators, buf, n, err)
	binary.WriteBinary(s.UnbondingValidators, buf, n, err)
	binary.WriteByteSlice(s.accounts.Hash(), buf, n, err)
	binary.WriteByteSlice(s.validatorInfos.Hash(), buf, n, err)
	if *err != nil {
		panic(*err)
	}
	s.DB.Set(stateKey, buf.Bytes())
}

// CONTRACT:
// Copy() is a cheap way to take a snapshot,
// as if State were copied by value.
func (s *State) Copy() *State {
	return &State{
		DB:                   s.DB,
		ChainID:              s.ChainID,
		LastBlockHeight:      s.LastBlockHeight,
		LastBlockHash:        s.LastBlockHash,
		LastBlockParts:       s.LastBlockParts,
		LastBlockTime:        s.LastBlockTime,
		BondedValidators:     s.BondedValidators.Copy(),     // TODO remove need for Copy() here.
		LastBondedValidators: s.LastBondedValidators.Copy(), // That is, make updates to the validator set
		UnbondingValidators:  s.UnbondingValidators.Copy(),  // copy the valSet lazily.
		accounts:             s.accounts.Copy(),
		validatorInfos:       s.validatorInfos.Copy(),
		evc:                  nil,
	}
}

// Returns a hash that represents the state data, excluding Last*
func (s *State) Hash() []byte {
	hashables := []merkle.Hashable{
		s.BondedValidators,
		s.UnbondingValidators,
		s.accounts,
		s.validatorInfos,
	}
	return merkle.HashFromHashables(hashables)
}

// Mutates the block in place and updates it with new state hash.
func (s *State) SetBlockStateHash(block *types.Block) error {
	sCopy := s.Copy()
	// sCopy has no event cache in it, so this won't fire events
	err := execBlock(sCopy, block, types.PartSetHeader{})
	if err != nil {
		return err
	}
	// Set block.StateHash
	block.StateHash = sCopy.Hash()
	return nil
}

//-------------------------------------
// State.accounts

// The returned Account is a copy, so mutating it
// has no side effects.
// Implements Statelike
func (s *State) GetAccount(address []byte) *account.Account {
	_, acc := s.accounts.Get(address)
	if acc == nil {
		return nil
	}
	return acc.(*account.Account).Copy()
}

// The account is copied before setting, so mutating it
// afterwards has no side effects.
// Implements Statelike
func (s *State) UpdateAccount(account *account.Account) bool {
	return s.accounts.Set(account.Address, account.Copy())
}

// Implements Statelike
func (s *State) RemoveAccount(address []byte) bool {
	_, removed := s.accounts.Remove(address)
	return removed
}

// The returned Account is a copy, so mutating it
// has no side effects.
func (s *State) GetAccounts() merkle.Tree {
	return s.accounts.Copy()
}

// State.accounts
//-------------------------------------
// State.validators

// The returned ValidatorInfo is a copy, so mutating it
// has no side effects.
func (s *State) GetValidatorInfo(address []byte) *ValidatorInfo {
	_, valInfo := s.validatorInfos.Get(address)
	if valInfo == nil {
		return nil
	}
	return valInfo.(*ValidatorInfo).Copy()
}

// Returns false if new, true if updated.
// The valInfo is copied before setting, so mutating it
// afterwards has no side effects.
func (s *State) SetValidatorInfo(valInfo *ValidatorInfo) (updated bool) {
	return s.validatorInfos.Set(valInfo.Address, valInfo.Copy())
}

func (s *State) unbondValidator(val *Validator) {
	// Move validator to UnbondingValidators
	val, removed := s.BondedValidators.Remove(val.Address)
	if !removed {
		panic("Couldn't remove validator for unbonding")
	}
	val.UnbondHeight = s.LastBlockHeight + 1
	added := s.UnbondingValidators.Add(val)
	if !added {
		panic("Couldn't add validator for unbonding")
	}
}

func (s *State) rebondValidator(val *Validator) {
	// Move validator to BondingValidators
	val, removed := s.UnbondingValidators.Remove(val.Address)
	if !removed {
		panic("Couldn't remove validator for rebonding")
	}
	val.BondHeight = s.LastBlockHeight + 1
	added := s.BondedValidators.Add(val)
	if !added {
		panic("Couldn't add validator for rebonding")
	}
}

func (s *State) releaseValidator(val *Validator) {
	// Update validatorInfo
	valInfo := s.GetValidatorInfo(val.Address)
	if valInfo == nil {
		panic("Couldn't find validatorInfo for release")
	}
	valInfo.ReleasedHeight = s.LastBlockHeight + 1
	s.SetValidatorInfo(valInfo)

	// Send coins back to UnbondTo outputs
	accounts, err := getOrMakeAccounts(s, nil, valInfo.UnbondTo)
	if err != nil {
		panic("Couldn't get or make unbondTo accounts")
	}
	adjustByOutputs(accounts, valInfo.UnbondTo)
	for _, acc := range accounts {
		s.UpdateAccount(acc)
	}

	// Remove validator from UnbondingValidators
	_, removed := s.UnbondingValidators.Remove(val.Address)
	if !removed {
		panic("Couldn't remove validator for release")
	}
}

func (s *State) destroyValidator(val *Validator) {
	// Update validatorInfo
	valInfo := s.GetValidatorInfo(val.Address)
	if valInfo == nil {
		panic("Couldn't find validatorInfo for release")
	}
	valInfo.DestroyedHeight = s.LastBlockHeight + 1
	valInfo.DestroyedAmount = val.VotingPower
	s.SetValidatorInfo(valInfo)

	// Remove validator
	_, removed := s.BondedValidators.Remove(val.Address)
	if !removed {
		_, removed := s.UnbondingValidators.Remove(val.Address)
		if !removed {
			panic("Couldn't remove validator for destruction")
		}
	}

}

// State.validators
//-------------------------------------
// State.storage

func (s *State) LoadStorage(hash []byte) (storage merkle.Tree) {
	storage = merkle.NewIAVLTree(binary.BasicCodec, binary.BasicCodec, 1024, s.DB)
	storage.Load(hash)
	return storage
}

// State.storage
//-------------------------------------

// Implements events.Eventable. Typically uses events.EventCache
func (s *State) SetFireable(evc events.Fireable) {
	s.evc = evc
}

//-----------------------------------------------------------------------------

type InvalidTxError struct {
	Tx     types.Tx
	Reason error
}

func (txErr InvalidTxError) Error() string {
	return fmt.Sprintf("Invalid tx: [%v] reason: [%v]", txErr.Tx, txErr.Reason)
}
