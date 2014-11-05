package state

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

var (
	ErrStateInvalidAccountId      = errors.New("Error State invalid account id")
	ErrStateInvalidSignature      = errors.New("Error State invalid signature")
	ErrStateInvalidSequenceNumber = errors.New("Error State invalid sequence number")
	ErrStateInvalidAccountState   = errors.New("Error State invalid account state")
	ErrStateInsufficientFunds     = errors.New("Error State insufficient funds")

	stateKey                           = []byte("stateKey")
	minBondAmount                      = uint64(1)             // TODO adjust
	defaultAccountDetailsCacheCapacity = 1000                  // TODO adjust
	unbondingPeriodBlocks              = uint32(60 * 24 * 365) // TODO probably better to make it time based.
	validatorTimeoutBlocks             = uint32(10)            // TODO adjust
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
	LastBlockHeight     uint32
	LastBlockHash       []byte
	LastBlockParts      PartSetHeader
	LastBlockTime       time.Time
	BondedValidators    *ValidatorSet
	UnbondingValidators *ValidatorSet
	accountDetails      merkle.Tree // Shouldn't be accessed directly.
}

func LoadState(db db_.DB) *State {
	s := &State{DB: db}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		return nil
	} else {
		reader := bytes.NewReader(buf)
		var n int64
		var err error
		s.LastBlockHeight = ReadUInt32(reader, &n, &err)
		s.LastBlockHash = ReadByteSlice(reader, &n, &err)
		s.LastBlockParts = ReadPartSetHeader(reader, &n, &err)
		s.LastBlockTime = ReadTime(reader, &n, &err)
		s.BondedValidators = ReadValidatorSet(reader, &n, &err)
		s.UnbondingValidators = ReadValidatorSet(reader, &n, &err)
		accountDetailsHash := ReadByteSlice(reader, &n, &err)
		s.accountDetails = merkle.NewIAVLTree(BasicCodec, AccountDetailCodec, defaultAccountDetailsCacheCapacity, db)
		s.accountDetails.Load(accountDetailsHash)
		if err != nil {
			panic(err)
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}

// Save this state into the db.
func (s *State) Save() {
	s.accountDetails.Save()
	var buf bytes.Buffer
	var n int64
	var err error
	WriteUInt32(&buf, s.LastBlockHeight, &n, &err)
	WriteByteSlice(&buf, s.LastBlockHash, &n, &err)
	WriteBinary(&buf, s.LastBlockParts, &n, &err)
	WriteTime(&buf, s.LastBlockTime, &n, &err)
	WriteBinary(&buf, s.BondedValidators, &n, &err)
	WriteBinary(&buf, s.UnbondingValidators, &n, &err)
	WriteByteSlice(&buf, s.accountDetails.Hash(), &n, &err)
	if err != nil {
		panic(err)
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
		accountDetails:      s.accountDetails.Copy(),
	}
}

// If the tx is invalid, an error will be returned.
// Unlike AppendBlock(), state will not be altered.
func (s *State) ExecTx(tx Tx) error {
	accDet := s.GetAccountDetail(tx.GetSignature().SignerId)
	if accDet == nil {
		return ErrStateInvalidAccountId
	}
	// Check signature
	if !accDet.Verify(tx) {
		return ErrStateInvalidSignature
	}
	// Check and update sequence
	if tx.GetSequence() <= accDet.Sequence {
		return ErrStateInvalidSequenceNumber
	} else {
		// TODO consider prevSequence for tx chaining.
		accDet.Sequence = tx.GetSequence()
	}
	// Subtract fee from balance.
	if accDet.Balance < tx.GetFee() {
		return ErrStateInsufficientFunds
	} else {
		accDet.Balance -= tx.GetFee()
	}
	// Exec tx
	switch tx.(type) {
	case *SendTx:
		stx := tx.(*SendTx)
		toAccDet := s.GetAccountDetail(stx.To)
		// Accounts must be nominal
		if accDet.Status != AccountStatusNominal {
			return ErrStateInvalidAccountState
		}
		if toAccDet.Status != AccountStatusNominal {
			return ErrStateInvalidAccountState
		}
		// Check account balance
		if accDet.Balance < stx.Amount {
			return ErrStateInsufficientFunds
		}
		// Check existence of destination account
		if toAccDet == nil {
			return ErrStateInvalidAccountId
		}
		// Good!
		accDet.Balance -= stx.Amount
		toAccDet.Balance += stx.Amount
		s.SetAccountDetail(accDet)
		s.SetAccountDetail(toAccDet)
		return nil
	//case *NameTx
	case *BondTx:
		//btx := tx.(*BondTx)
		// Account must be nominal
		if accDet.Status != AccountStatusNominal {
			return ErrStateInvalidAccountState
		}
		// Check account balance
		if accDet.Balance < minBondAmount {
			return ErrStateInsufficientFunds
		}
		// Good!
		accDet.Status = AccountStatusBonded
		s.SetAccountDetail(accDet)
		added := s.BondedValidators.Add(&Validator{
			Account:     accDet.Account,
			BondHeight:  s.LastBlockHeight,
			VotingPower: accDet.Balance,
			Accum:       0,
		})
		if !added {
			panic("Failed to add validator")
		}
		return nil
	case *UnbondTx:
		//utx := tx.(*UnbondTx)
		// Account must be bonded.
		if accDet.Status != AccountStatusBonded {
			return ErrStateInvalidAccountState
		}
		// Good!
		s.unbondValidator(accDet.Id, accDet)
		s.SetAccountDetail(accDet)
		return nil
	case *DupeoutTx:
		{
			// NOTE: accDet is the one who created this transaction.
			// Subtract any fees, save, and forget.
			s.SetAccountDetail(accDet)
			accDet = nil
		}
		dtx := tx.(*DupeoutTx)
		// Verify the signatures
		if dtx.VoteA.SignerId != dtx.VoteB.SignerId {
			return ErrStateInvalidSignature
		}
		accused := s.GetAccountDetail(dtx.VoteA.SignerId)
		if !accused.Verify(&dtx.VoteA) || !accused.Verify(&dtx.VoteB) {
			return ErrStateInvalidSignature
		}
		// Verify equivocation
		if dtx.VoteA.Height != dtx.VoteB.Height {
			return errors.New("DupeoutTx height must be the same.")
		}
		if dtx.VoteA.Type == VoteTypeCommit && dtx.VoteA.Round < dtx.VoteB.Round {
			// Check special case.
			// Validators should not sign another vote after committing.
		} else {
			if dtx.VoteA.Round != dtx.VoteB.Round {
				return errors.New("DupeoutTx rounds don't match")
			}
			if dtx.VoteA.Type != dtx.VoteB.Type {
				return errors.New("DupeoutTx types don't match")
			}
			if bytes.Equal(dtx.VoteA.BlockHash, dtx.VoteB.BlockHash) {
				return errors.New("DupeoutTx blockhash shouldn't match")
			}
		}
		// Good! (Bad validator!)
		if accused.Status == AccountStatusBonded {
			_, removed := s.BondedValidators.Remove(accused.Id)
			if !removed {
				panic("Failed to remove accused validator")
			}
		} else if accused.Status == AccountStatusUnbonding {
			_, removed := s.UnbondingValidators.Remove(accused.Id)
			if !removed {
				panic("Failed to remove accused validator")
			}
		} else {
			panic("Couldn't find accused validator")
		}
		accused.Status = AccountStatusDupedOut
		updated := s.SetAccountDetail(accused)
		if !updated {
			panic("Failed to update accused validator account")
		}
		return nil
	default:
		panic("Unknown Tx type")
	}
}

// accDet optional
func (s *State) unbondValidator(accountId uint64, accDet *AccountDetail) {
	if accDet == nil {
		accDet = s.GetAccountDetail(accountId)
	}
	accDet.Status = AccountStatusUnbonding
	s.SetAccountDetail(accDet)
	val, removed := s.BondedValidators.Remove(accDet.Id)
	if !removed {
		panic("Failed to remove validator")
	}
	val.UnbondHeight = s.LastBlockHeight
	added := s.UnbondingValidators.Add(val)
	if !added {
		panic("Failed to add validator")
	}
}

func (s *State) releaseValidator(accountId uint64) {
	accDet := s.GetAccountDetail(accountId)
	if accDet.Status != AccountStatusUnbonding {
		panic("Cannot release validator")
	}
	accDet.Status = AccountStatusNominal
	// TODO: move balance to designated address, UnbondTo.
	s.SetAccountDetail(accDet)
	_, removed := s.UnbondingValidators.Remove(accountId)
	if !removed {
		panic("Couldn't release validator")
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
			rsig := block.Validation.Commits[index]
			if rsig.IsZero() {
				return false
			} else {
				if rsig.SignerId != val.Id {
					err = errors.New("Invalid validation order")
					return true
				}
				vote := &Vote{
					Height:     block.Height - 1,
					Round:      rsig.Round,
					Type:       VoteTypeCommit,
					BlockHash:  block.LastBlockHash,
					BlockParts: block.LastBlockParts,
					Signature:  rsig.Signature,
				}
				if val.Verify(vote) {
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
	for _, rsig := range block.Validation.Commits {
		_, val := s.BondedValidators.GetById(rsig.SignerId)
		if val == nil {
			return ErrStateInvalidSignature
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
		s.releaseValidator(val.Id)
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
		s.unbondValidator(val.Id, nil)
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

// The returned AccountDetail is a copy, so mutating it
// has no side effects.
func (s *State) GetAccountDetail(accountId uint64) *AccountDetail {
	_, accDet := s.accountDetails.Get(accountId)
	if accDet == nil {
		return nil
	}
	return accDet.(*AccountDetail).Copy()
}

// Returns false if new, true if updated.
// The accDet is copied before setting, so mutating it
// afterwards has no side effects.
func (s *State) SetAccountDetail(accDet *AccountDetail) (updated bool) {
	return s.accountDetails.Set(accDet.Id, accDet.Copy())
}

// Returns a hash that represents the state data,
// excluding LastBlock*
func (s *State) Hash() []byte {
	hashables := []merkle.Hashable{
		s.BondedValidators,
		s.UnbondingValidators,
		s.accountDetails,
	}
	return merkle.HashFromHashables(hashables)
}
