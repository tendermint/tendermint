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
	ErrStateInvalidSequenceNumber      = errors.New("Error State invalid sequence number")
	ErrStateInvalidValidationStateHash = errors.New("Error State invalid ValidationStateHash")
	ErrStateInvalidAccountStateHash    = errors.New("Error State invalid AccountStateHash")

	stateKey = []byte("stateKey")
)

type accountBalanceCodec struct{}

func (abc accountBalanceCodec) Write(accBal interface{}) (accBalBytes []byte, err error) {
	w := new(bytes.Buffer)
	_, err = accBal.(*AccountBalance).WriteTo(w)
	return w.Bytes(), err
}

func (abc accountBalanceCodec) Read(accBalBytes []byte) (interface{}, error) {
	n, err, r := new(int64), new(error), bytes.NewBuffer(accBalBytes)
	return ReadAccountBalance(r, n, err), *err
}

//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	DB              DB
	Height          uint32 // Last known block height
	BlockHash       []byte // Last known block hash
	CommitTime      time.Time
	AccountBalances *merkle.TypedTree
	Validators      *ValidatorSet
}

func GenesisState(db DB, genesisTime time.Time, accBals []*AccountBalance) *State {

	// TODO: Use "uint64Codec" instead of BasicCodec
	accountBalances := merkle.NewTypedTree(merkle.NewIAVLTree(db), BasicCodec, accountBalanceCodec{})
	validators := map[uint64]*Validator{}

	for _, accBal := range accBals {
		accountBalances.Set(accBal.Id, accBal)
		validators[accBal.Id] = &Validator{
			Account:     accBal.Account,
			BondHeight:  0,
			VotingPower: accBal.Balance,
			Accum:       0,
		}
	}
	validatorSet := NewValidatorSet(validators)

	return &State{
		DB:              db,
		Height:          0,
		BlockHash:       nil,
		CommitTime:      genesisTime,
		AccountBalances: accountBalances,
		Validators:      validatorSet,
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
		accountBalancesHash := ReadByteSlice(reader, &n, &err)
		s.AccountBalances = merkle.NewTypedTree(merkle.LoadIAVLTreeFromHash(db, accountBalancesHash), BasicCodec, accountBalanceCodec{})
		var validators = map[uint64]*Validator{}
		for reader.Len() > 0 {
			validator := ReadValidator(reader, &n, &err)
			validators[validator.Id] = validator
		}
		s.Validators = NewValidatorSet(validators)
		if err != nil {
			panic(err)
		}
	}
	return s
}

// Save this state into the db.
// For convenience, the commitTime (required by ConsensusAgent)
// is saved here.
func (s *State) Save(commitTime time.Time) {
	s.CommitTime = commitTime
	s.AccountBalances.Tree.Save()
	var buf bytes.Buffer
	var n int64
	var err error
	WriteUInt32(&buf, s.Height, &n, &err)
	WriteTime(&buf, commitTime, &n, &err)
	WriteByteSlice(&buf, s.BlockHash, &n, &err)
	WriteByteSlice(&buf, s.AccountBalances.Tree.Hash(), &n, &err)
	for _, validator := range s.Validators.Map() {
		WriteBinary(&buf, validator, &n, &err)
	}
	if err != nil {
		panic(err)
	}
	s.DB.Set(stateKey, buf.Bytes())
}

func (s *State) Copy() *State {
	return &State{
		DB:              s.DB,
		Height:          s.Height,
		CommitTime:      s.CommitTime,
		BlockHash:       s.BlockHash,
		AccountBalances: s.AccountBalances.Copy(),
		Validators:      s.Validators.Copy(),
	}
}

// If the tx is invalid, an error will be returned.
// Unlike AppendBlock(), state will not be altered.
func (s *State) ExecTx(tx Tx) error {
	/*
		// Get the signer's incr
		signerId := tx.Signature().SignerId
		if mem.state.AccountSequence(signerId) != tx.Sequence() {
			return ErrStateInvalidSequenceNumber
		}
	*/
	// XXX commit the tx
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

	// Increment validator AccumPowers
	s.Validators.IncrementAccum()

	// State hashes should match
	if !bytes.Equal(s.Validators.Hash(), b.ValidationStateHash) {
		return ErrStateInvalidValidationStateHash
	}
	if !bytes.Equal(s.AccountBalances.Tree.Hash(), b.AccountStateHash) {
		return ErrStateInvalidAccountStateHash
	}

	s.Height = b.Height
	s.BlockHash = b.Hash()
	return nil
}

func (s *State) AccountBalance(accountId uint64) *AccountBalance {
	accBal := s.AccountBalances.Get(accountId)
	if accBal == nil {
		return nil
	}
	return accBal.(*AccountBalance)
}
