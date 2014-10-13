package state

import (
	"bytes"
	"errors"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

var (
	ErrStateInvalidAccountId           = errors.New("Error State invalid account id")
	ErrStateInvalidSignature           = errors.New("Error State invalid signature")
	ErrStateInvalidSequenceNumber      = errors.New("Error State invalid sequence number")
	ErrStateInvalidAccountState        = errors.New("Error State invalid account state")
	ErrStateInvalidValidationStateHash = errors.New("Error State invalid ValidationStateHash")
	ErrStateInvalidAccountStateHash    = errors.New("Error State invalid AccountStateHash")
	ErrStateInsufficientFunds          = errors.New("Error State insufficient funds")

	stateKey                           = []byte("stateKey")
	minBondAmount                      = uint64(1) // TODO adjust
	defaultAccountDetailsCacheCapacity = 1000      // TODO adjust
)

//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	DB                 DB
	Height             uint32 // Last known block height
	BlockHash          []byte // Last known block hash
	CommitTime         time.Time
	AccountDetails     merkle.Tree
	BondedValidators   *ValidatorSet
	UnbondedValidators *ValidatorSet
}

func GenesisState(db DB, genesisTime time.Time, accDets []*AccountDetail) *State {

	// TODO: Use "uint64Codec" instead of BasicCodec
	accountDetails := merkle.NewIAVLTree(BasicCodec, AccountDetailCodec, defaultAccountDetailsCacheCapacity, db)
	validators := []*Validator{}

	for _, accDet := range accDets {
		accountDetails.Set(accDet.Id, accDet)
		if accDet.Status == AccountDetailStatusBonded {
			validators = append(validators, &Validator{
				Account:     accDet.Account,
				BondHeight:  0,
				VotingPower: accDet.Balance,
				Accum:       0,
			})
		}
	}

	if len(validators) == 0 {
		panic("Must have some validators")
	}

	return &State{
		DB:                 db,
		Height:             0,
		BlockHash:          nil,
		CommitTime:         genesisTime,
		AccountDetails:     accountDetails,
		BondedValidators:   NewValidatorSet(validators),
		UnbondedValidators: NewValidatorSet(nil),
	}
}

func LoadState(db DB) *State {
	s := &State{DB: db}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		return nil
	} else {
		reader := bytes.NewReader(buf)
		var n int64
		var err error
		s.Height = ReadUInt32(reader, &n, &err)
		s.CommitTime = ReadTime(reader, &n, &err)
		s.BlockHash = ReadByteSlice(reader, &n, &err)
		accountDetailsHash := ReadByteSlice(reader, &n, &err)
		s.AccountDetails = merkle.NewIAVLTree(BasicCodec, AccountDetailCodec, defaultAccountDetailsCacheCapacity, db)
		s.AccountDetails.Load(accountDetailsHash)
		s.BondedValidators = ReadValidatorSet(reader, &n, &err)
		s.UnbondedValidators = ReadValidatorSet(reader, &n, &err)
		if err != nil {
			panic(err)
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}

// Save this state into the db.
// For convenience, the commitTime (required by ConsensusAgent)
// is saved here.
func (s *State) Save(commitTime time.Time) {
	s.CommitTime = commitTime
	s.AccountDetails.Save()
	var buf bytes.Buffer
	var n int64
	var err error
	WriteUInt32(&buf, s.Height, &n, &err)
	WriteTime(&buf, commitTime, &n, &err)
	WriteByteSlice(&buf, s.BlockHash, &n, &err)
	WriteByteSlice(&buf, s.AccountDetails.Hash(), &n, &err)
	WriteBinary(&buf, s.BondedValidators, &n, &err)
	WriteBinary(&buf, s.UnbondedValidators, &n, &err)
	if err != nil {
		panic(err)
	}
	s.DB.Set(stateKey, buf.Bytes())
}

func (s *State) Copy() *State {
	return &State{
		DB:                 s.DB,
		Height:             s.Height,
		CommitTime:         s.CommitTime,
		BlockHash:          s.BlockHash,
		AccountDetails:     s.AccountDetails.Copy(),
		BondedValidators:   s.BondedValidators.Copy(),
		UnbondedValidators: s.UnbondedValidators.Copy(),
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
	// Check sequence
	if tx.GetSequence() <= accDet.Sequence {
		return ErrStateInvalidSequenceNumber
	}
	// Exec tx
	switch tx.(type) {
	case *SendTx:
		stx := tx.(*SendTx)
		toAccDet := s.GetAccountDetail(stx.To)
		// Accounts must be nominal
		if accDet.Status != AccountDetailStatusNominal {
			return ErrStateInvalidAccountState
		}
		if toAccDet.Status != AccountDetailStatusNominal {
			return ErrStateInvalidAccountState
		}
		// Check account balance
		if accDet.Balance < stx.Fee+stx.Amount {
			return ErrStateInsufficientFunds
		}
		// Check existence of destination account
		if toAccDet == nil {
			return ErrStateInvalidAccountId
		}
		// Good!
		accDet.Balance -= (stx.Fee + stx.Amount)
		toAccDet.Balance += (stx.Amount)
		s.SetAccountDetail(accDet)
		s.SetAccountDetail(toAccDet)
	//case *NameTx
	case *BondTx:
		btx := tx.(*BondTx)
		// Account must be nominal
		if accDet.Status != AccountDetailStatusNominal {
			return ErrStateInvalidAccountState
		}
		// Check account balance
		if accDet.Balance < minBondAmount {
			return ErrStateInsufficientFunds
		}
		// TODO: max number of validators?
		// Good!
		accDet.Balance -= btx.Fee // remaining balance are bonded coins.
		accDet.Status = AccountDetailStatusBonded
		s.SetAccountDetail(accDet)
		added := s.BondednValidators.Add(&Validator{
			Account:     accDet.Account,
			BondHeight:  s.Height,
			VotingPower: accDet.Balance,
			Accum:       0,
		})
		if !added {
			panic("Failed to add validator")
		}
	case *UnbondTx:
		utx := tx.(*UnbondTx)
		// Account must be bonded.
		if accDet.Status != AccountDetailStatusBonded {
			return ErrStateInvalidAccountState
		}
		// Good!
		accDet.Status = AccountDetailStatusUnbonding
		s.SetAccountDetail(accDet)
		val, removed := s.BondedValidators.Remove(accDet.Id)
		if !removed {
			panic("Failed to remove validator")
		}
		val.UnbondHeight = s.Height
		added := s.UnbondedValidators.Add(val)
		if !added {
			panic("Failed to add validator")
		}
	case *DupeoutTx:
		// XXX
	}
	panic("Implement ExecTx()")
	return nil
}

// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling AppendBlock!
func (s *State) AppendBlock(b *Block) error {
	// Basic block validation.
	err := b.ValidateBasic(s.Height, s.BlockHash)
	if err != nil {
		return err
	}

	// Commit each tx
	for _, tx := range b.Data.Txs {
		err := s.ExecTx(tx)
		if err != nil {
			return err
		}
	}

	// If any unbonding periods are over,
	// reward account with bonded coins.

	// If any validators haven't signed in a while,
	// unbond them, they have timed out.

	// Increment validator AccumPowers
	s.BondedValidators.IncrementAccum()

	// State hashes should match
	if !bytes.Equal(s.BondedValidators.Hash(), b.ValidationStateHash) {
		return ErrStateInvalidValidationStateHash
	}
	if !bytes.Equal(s.AccountDetails.Hash(), b.AccountStateHash) {
		return ErrStateInvalidAccountStateHash
	}

	s.Height = b.Height
	s.BlockHash = b.Hash()
	return nil
}

func (s *State) GetAccountDetail(accountId uint64) *AccountDetail {
	_, accDet := s.AccountDetails.Get(accountId)
	if accDet == nil {
		return nil
	}
	return accDet.(*AccountDetail)
}

// Returns false if new, true if updated.
func (s *State) SetAccountDetail(accDet *AccountDetail) (updated bool) {
	return s.AccountDetails.Set(accDet.Id, accDet)
}
