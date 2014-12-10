package state

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

var (
	stateKey                     = []byte("stateKey")
	minBondAmount                = uint64(1)           // TODO adjust
	defaultAccountsCacheCapacity = 1000                // TODO adjust
	unbondingPeriodBlocks        = uint(60 * 24 * 365) // TODO probably better to make it time based.
	validatorTimeoutBlocks       = uint(10)            // TODO adjust
)

//-----------------------------------------------------------------------------

type InvalidTxError struct {
	Tx     Tx
	Reason error
}

func (txErr InvalidTxError) Error() string {
	return fmt.Sprintf("Invalid tx: [%v] reason: [%v]", txErr.Tx, txErr.Reason)
}

//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	DB                  db_.DB
	LastBlockHeight     uint
	LastBlockHash       []byte
	LastBlockParts      PartSetHeader
	LastBlockTime       time.Time
	BondedValidators    *ValidatorSet
	UnbondingValidators *ValidatorSet
	accounts            merkle.Tree // Shouldn't be accessed directly.
	validatorInfos      merkle.Tree // Shouldn't be accessed directly.
}

func LoadState(db db_.DB) *State {
	s := &State{DB: db}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int64), new(error)
		s.LastBlockHeight = ReadUVarInt(r, n, err)
		s.LastBlockHash = ReadByteSlice(r, n, err)
		s.LastBlockParts = ReadBinary(PartSetHeader{}, r, n, err).(PartSetHeader)
		s.LastBlockTime = ReadTime(r, n, err)
		s.BondedValidators = ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		s.UnbondingValidators = ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		accountsHash := ReadByteSlice(r, n, err)
		s.accounts = merkle.NewIAVLTree(BasicCodec, AccountCodec, defaultAccountsCacheCapacity, db)
		s.accounts.Load(accountsHash)
		validatorInfosHash := ReadByteSlice(r, n, err)
		s.validatorInfos = merkle.NewIAVLTree(BasicCodec, ValidatorInfoCodec, 0, db)
		s.validatorInfos.Load(validatorInfosHash)
		if *err != nil {
			panic(*err)
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}

// Save this state into the db.
func (s *State) Save() {
	s.accounts.Save()
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	WriteUVarInt(s.LastBlockHeight, buf, n, err)
	WriteByteSlice(s.LastBlockHash, buf, n, err)
	WriteBinary(s.LastBlockParts, buf, n, err)
	WriteTime(s.LastBlockTime, buf, n, err)
	WriteBinary(s.BondedValidators, buf, n, err)
	WriteBinary(s.UnbondingValidators, buf, n, err)
	WriteByteSlice(s.accounts.Hash(), buf, n, err)
	WriteByteSlice(s.validatorInfos.Hash(), buf, n, err)
	if *err != nil {
		panic(*err)
	}
	s.DB.Set(stateKey, buf.Bytes())
}

func (s *State) Copy() *State {
	return &State{
		DB:                  s.DB,
		LastBlockHeight:     s.LastBlockHeight,
		LastBlockHash:       s.LastBlockHash,
		LastBlockParts:      s.LastBlockParts,
		LastBlockTime:       s.LastBlockTime,
		BondedValidators:    s.BondedValidators.Copy(),
		UnbondingValidators: s.UnbondingValidators.Copy(),
		accounts:            s.accounts.Copy(),
		validatorInfos:      s.validatorInfos.Copy(),
	}
}

func (s *State) GetOrMakeAccounts(ins []*TxInput, outs []*TxOutput) (map[string]*Account, error) {
	accounts := map[string]*Account{}
	for _, in := range ins {
		// Account shouldn't be duplicated
		if _, ok := accounts[string(in.Address)]; ok {
			return nil, ErrTxDuplicateAddress
		}
		account := s.GetAccount(in.Address)
		if account == nil {
			return nil, ErrTxInvalidAddress
		}
		accounts[string(in.Address)] = account
	}
	for _, out := range outs {
		// Account shouldn't be duplicated
		if _, ok := accounts[string(out.Address)]; ok {
			return nil, ErrTxDuplicateAddress
		}
		account := s.GetAccount(out.Address)
		// output account may be nil (new)
		if account == nil {
			account = NewAccount(out.Address, PubKeyUnknown{})
		}
		accounts[string(out.Address)] = account
	}
	return accounts, nil
}

func (s *State) ValidateInputs(accounts map[string]*Account, signBytes []byte, ins []*TxInput) (total uint64, err error) {
	for _, in := range ins {
		account := accounts[string(in.Address)]
		if account == nil {
			panic("ValidateInputs() expects account in accounts")
		}
		// Check TxInput basic
		if err := in.ValidateBasic(); err != nil {
			return 0, err
		}
		// Check amount
		if account.Balance < in.Amount {
			return 0, ErrTxInsufficientFunds
		}
		// Check signatures
		if !account.PubKey.VerifyBytes(signBytes, in.Signature) {
			return 0, ErrTxInvalidSignature
		}
		// Check sequences
		if account.Sequence+1 != in.Sequence {
			return 0, ErrTxInvalidSequence
		}
		// Good. Add amount to total
		total += in.Amount
	}
	return total, nil
}

func (s *State) ValidateOutputs(outs []*TxOutput) (total uint64, err error) {
	for _, out := range outs {
		// Check TxOutput basic
		if err := out.ValidateBasic(); err != nil {
			return 0, err
		}
		// Good. Add amount to total
		total += out.Amount
	}
	return total, nil
}

func (s *State) AdjustByInputs(accounts map[string]*Account, ins []*TxInput) {
	for _, in := range ins {
		account := accounts[string(in.Address)]
		if account == nil {
			panic("AdjustByInputs() expects account in accounts")
		}
		if account.Balance < in.Amount {
			panic("AdjustByInputs() expects sufficient funds")
		}
		account.Balance -= in.Amount
	}
}

func (s *State) AdjustByOutputs(accounts map[string]*Account, outs []*TxOutput) {
	for _, out := range outs {
		account := accounts[string(out.Address)]
		if account == nil {
			panic("AdjustByInputs() expects account in accounts")
		}
		account.Balance += out.Amount
	}
}

// If the tx is invalid, an error will be returned.
// Unlike AppendBlock(), state will not be altered.
func (s *State) ExecTx(tx_ Tx) error {

	// TODO: do something with fees
	fees := uint64(0)

	// Exec tx
	switch tx_.(type) {
	case *SendTx:
		tx := tx_.(*SendTx)
		accounts, err := s.GetOrMakeAccounts(tx.Inputs, tx.Outputs)
		if err != nil {
			return err
		}
		signBytes := SignBytes(tx)
		inTotal, err := s.ValidateInputs(accounts, signBytes, tx.Inputs)
		if err != nil {
			return err
		}
		outTotal, err := s.ValidateOutputs(tx.Outputs)
		if err != nil {
			return err
		}
		if outTotal > inTotal {
			return ErrTxInsufficientFunds
		}
		fee := inTotal - outTotal
		fees += fee

		// Good! Adjust accounts
		s.AdjustByInputs(accounts, tx.Inputs)
		s.AdjustByOutputs(accounts, tx.Outputs)
		s.SetAccounts(accounts)
		return nil

	case *BondTx:
		tx := tx_.(*BondTx)
		accounts, err := s.GetOrMakeAccounts(tx.Inputs, tx.UnbondTo)
		if err != nil {
			return err
		}
		signBytes := SignBytes(tx)
		inTotal, err := s.ValidateInputs(accounts, signBytes, tx.Inputs)
		if err != nil {
			return err
		}
		if err := tx.PubKey.ValidateBasic(); err != nil {
			return err
		}
		outTotal, err := s.ValidateOutputs(tx.UnbondTo)
		if err != nil {
			return err
		}
		if outTotal > inTotal {
			return ErrTxInsufficientFunds
		}
		fee := inTotal - outTotal
		fees += fee

		// Good! Adjust accounts
		s.AdjustByInputs(accounts, tx.Inputs)
		s.SetAccounts(accounts)
		// Add ValidatorInfo
		updated := s.SetValidatorInfo(&ValidatorInfo{
			Address:         tx.PubKey.Address(),
			PubKey:          tx.PubKey,
			UnbondTo:        tx.UnbondTo,
			FirstBondHeight: s.LastBlockHeight + 1,
		})
		if !updated {
			panic("Failed to add validator info")
		}
		// Add Validator
		added := s.BondedValidators.Add(&Validator{
			Address:     tx.PubKey.Address(),
			PubKey:      tx.PubKey,
			BondHeight:  s.LastBlockHeight + 1,
			VotingPower: inTotal,
			Accum:       0,
		})
		if !added {
			panic("Failed to add validator")
		}
		return nil

	case *UnbondTx:
		tx := tx_.(*UnbondTx)

		// The validator must be active
		_, val := s.BondedValidators.GetByAddress(tx.Address)
		if val == nil {
			return ErrTxInvalidAddress
		}

		// Verify the signature
		signBytes := SignBytes(tx)
		if !val.PubKey.VerifyBytes(signBytes, tx.Signature) {
			return ErrTxInvalidSignature
		}

		// tx.Height must be greater than val.LastCommitHeight
		if tx.Height < val.LastCommitHeight {
			return errors.New("Invalid bond height")
		}

		// Good!
		s.unbondValidator(val)
		return nil

	case *DupeoutTx:
		tx := tx_.(*DupeoutTx)

		// Verify the signatures
		_, accused := s.BondedValidators.GetByAddress(tx.Address)
		voteASignBytes := SignBytes(&tx.VoteA)
		voteBSignBytes := SignBytes(&tx.VoteB)
		if !accused.PubKey.VerifyBytes(voteASignBytes, tx.VoteA.Signature) ||
			!accused.PubKey.VerifyBytes(voteBSignBytes, tx.VoteB.Signature) {
			return ErrTxInvalidSignature
		}

		// Verify equivocation
		// TODO: in the future, just require one vote from a previous height that
		// doesn't exist on this chain.
		if tx.VoteA.Height != tx.VoteB.Height {
			return errors.New("DupeoutTx heights don't match")
		}
		if tx.VoteA.Type == VoteTypeCommit && tx.VoteA.Round < tx.VoteB.Round {
			// Check special case.
			// Validators should not sign another vote after committing.
		} else {
			if tx.VoteA.Round != tx.VoteB.Round {
				return errors.New("DupeoutTx rounds don't match")
			}
			if tx.VoteA.Type != tx.VoteB.Type {
				return errors.New("DupeoutTx types don't match")
			}
			if bytes.Equal(tx.VoteA.BlockHash, tx.VoteB.BlockHash) {
				return errors.New("DupeoutTx blockhashes shouldn't match")
			}
		}

		// Good! (Bad validator!)
		s.destroyValidator(accused)
		return nil

	default:
		panic("Unknown Tx type")
	}
}

func (s *State) unbondValidator(val *Validator) {
	// Move validator to UnbondingValidators
	val, removed := s.BondedValidators.Remove(val.Address)
	if !removed {
		panic("Couldn't remove validator for unbonding")
	}
	val.UnbondHeight = s.LastBlockHeight
	added := s.UnbondingValidators.Add(val)
	if !added {
		panic("Couldn't add validator for unbonding")
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
	accounts, err := s.GetOrMakeAccounts(nil, valInfo.UnbondTo)
	if err != nil {
		panic("Couldn't get or make unbondTo accounts")
	}
	s.AdjustByOutputs(accounts, valInfo.UnbondTo)
	s.SetAccounts(accounts)

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

// "checkStateHash": If false, instead of checking the resulting
// state.Hash() against block.StateHash, it *sets* the block.StateHash.
// (used for constructing a new proposal)
// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling AppendBlock!
func (s *State) AppendBlock(block *Block, blockPartsHeader PartSetHeader, checkStateHash bool) error {
	// Basic block validation.
	err := block.ValidateBasic(s.LastBlockHeight, s.LastBlockHash, s.LastBlockParts, s.LastBlockTime)
	if err != nil {
		return err
	}

	// Validate block Validation.
	if block.Height == 1 {
		if len(block.Validation.Commits) != 0 {
			return errors.New("Block at height 1 (first block) should have no Validation commits")
		}
	} else {
		if uint(len(block.Validation.Commits)) != s.BondedValidators.Size() {
			return errors.New("Invalid block validation size")
		}
		var sumVotingPower uint64
		s.BondedValidators.Iterate(func(index uint, val *Validator) bool {
			commit := block.Validation.Commits[index]
			if commit.IsZero() {
				return false
			} else {
				vote := &Vote{
					Height:     block.Height - 1,
					Round:      commit.Round,
					Type:       VoteTypeCommit,
					BlockHash:  block.LastBlockHash,
					BlockParts: block.LastBlockParts,
				}
				if val.PubKey.VerifyBytes(SignBytes(vote), commit.Signature) {
					sumVotingPower += val.VotingPower
					return false
				} else {
					log.Warning("Invalid validation signature.\nval: %v\nvote: %v", val, vote)
					err = errors.New("Invalid validation signature")
					return true
				}
			}
		})
		if err != nil {
			return err
		}
		if sumVotingPower <= s.BondedValidators.TotalVotingPower()*2/3 {
			return errors.New("Insufficient validation voting power")
		}
	}

	// Commit each tx
	for _, tx := range block.Data.Txs {
		err := s.ExecTx(tx)
		if err != nil {
			return InvalidTxError{tx, err}
		}
	}

	// Update Validator.LastCommitHeight as necessary.
	for i, commit := range block.Validation.Commits {
		if commit.IsZero() {
			continue
		}
		_, val := s.BondedValidators.GetByIndex(uint(i))
		if val == nil {
			return ErrTxInvalidSignature
		}
		val.LastCommitHeight = block.Height - 1
		updated := s.BondedValidators.Update(val)
		if !updated {
			panic("Failed to update validator LastCommitHeight")
		}
	}

	// If any unbonding periods are over,
	// reward account with bonded coins.
	toRelease := []*Validator{}
	s.UnbondingValidators.Iterate(func(index uint, val *Validator) bool {
		if val.UnbondHeight+unbondingPeriodBlocks < block.Height {
			toRelease = append(toRelease, val)
		}
		return false
	})
	for _, val := range toRelease {
		s.releaseValidator(val)
	}

	// If any validators haven't signed in a while,
	// unbond them, they have timed out.
	toTimeout := []*Validator{}
	s.BondedValidators.Iterate(func(index uint, val *Validator) bool {
		if val.LastCommitHeight+validatorTimeoutBlocks < block.Height {
			toTimeout = append(toTimeout, val)
		}
		return false
	})
	for _, val := range toTimeout {
		s.unbondValidator(val)
	}

	// Increment validator AccumPowers
	s.BondedValidators.IncrementAccum()

	// Check or set block.StateHash
	stateHash := s.Hash()
	if checkStateHash {
		// State hash should match
		if !bytes.Equal(stateHash, block.StateHash) {
			return Errorf("Invalid state hash. Got %X, block says %X",
				stateHash, block.StateHash)
		}
	} else {
		// Set the state hash.
		if block.StateHash != nil {
			panic("Cannot overwrite block.StateHash")
		}
		block.StateHash = stateHash
	}

	s.LastBlockHeight = block.Height
	s.LastBlockHash = block.Hash()
	s.LastBlockParts = blockPartsHeader
	s.LastBlockTime = block.Time
	return nil
}

// The returned Account is a copy, so mutating it
// has no side effects.
func (s *State) GetAccount(address []byte) *Account {
	_, account := s.accounts.Get(address)
	if account == nil {
		return nil
	}
	return account.(*Account).Copy()
}

// The accounts are copied before setting, so mutating it
// afterwards has no side effects.
func (s *State) SetAccounts(accounts map[string]*Account) {
	for _, account := range accounts {
		s.accounts.Set(account.Address, account.Copy())
	}
}

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

// Returns a hash that represents the state data,
// excluding LastBlock*
func (s *State) Hash() []byte {
	hashables := []merkle.Hashable{
		s.BondedValidators,
		s.UnbondingValidators,
		s.accounts,
	}
	return merkle.HashFromHashables(hashables)
}
