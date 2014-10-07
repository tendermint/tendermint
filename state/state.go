package state

import (
	"bytes"
	"errors"
	"sync"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

var (
	ErrStateInvalidSequenceNumber = errors.New("Error State invalid sequence number")

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

type State struct {
	mtx             sync.Mutex
	db              DB
	height          uint32 // Last known block height
	blockHash       []byte // Last known block hash
	commitTime      time.Time
	accountBalances *merkle.TypedTree
	validators      *ValidatorSet
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
		db:              db,
		height:          0,
		blockHash:       nil,
		commitTime:      genesisTime,
		accountBalances: accountBalances,
		validators:      validatorSet,
	}
}

func LoadState(db DB) *State {
	s := &State{db: db}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		return nil
	} else {
		reader := bytes.NewReader(buf)
		var n int64
		var err error
		s.height = ReadUInt32(reader, &n, &err)
		s.commitTime = ReadTime(reader, &n, &err)
		s.blockHash = ReadByteSlice(reader, &n, &err)
		accountBalancesHash := ReadByteSlice(reader, &n, &err)
		s.accountBalances = merkle.NewTypedTree(merkle.LoadIAVLTreeFromHash(db, accountBalancesHash), BasicCodec, accountBalanceCodec{})
		var validators = map[uint64]*Validator{}
		for reader.Len() > 0 {
			validator := ReadValidator(reader, &n, &err)
			validators[validator.Id] = validator
		}
		s.validators = NewValidatorSet(validators)
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
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.commitTime = commitTime
	s.accountBalances.Tree.Save()
	var buf bytes.Buffer
	var n int64
	var err error
	WriteUInt32(&buf, s.height, &n, &err)
	WriteTime(&buf, commitTime, &n, &err)
	WriteByteSlice(&buf, s.blockHash, &n, &err)
	WriteByteSlice(&buf, s.accountBalances.Tree.Hash(), &n, &err)
	for _, validator := range s.validators.Map() {
		WriteBinary(&buf, validator, &n, &err)
	}
	if err != nil {
		panic(err)
	}
	s.db.Set(stateKey, buf.Bytes())
}

func (s *State) Copy() *State {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return &State{
		db:              s.db,
		height:          s.height,
		commitTime:      s.commitTime,
		blockHash:       s.blockHash,
		accountBalances: s.accountBalances.Copy(),
		validators:      s.validators.Copy(),
	}
}

// If the tx is invalid, an error will be returned.
// Unlike AppendBlock(), state will not be altered.
func (s *State) ExecTx(tx Tx) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.execTx(tx)
}

func (s *State) execTx(tx Tx) error {
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
// at an invalid state.  Copy the state before calling Commit!
func (s *State) AppendBlock(b *Block) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Basic block validation.
	err := b.ValidateBasic(s.height, s.blockHash)
	if err != nil {
		return err
	}

	// Commit each tx
	for _, tx := range b.Data.Txs {
		err := s.execTx(tx)
		if err != nil {
			return err
		}
	}

	s.validators.IncrementAccum()
	s.height = b.Height
	s.blockHash = b.Hash()
	return nil
}

func (s *State) Height() uint32 {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.height
}

func (s *State) CommitTime() time.Time {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.commitTime
}

// The returned ValidatorSet gets mutated upon s.Commit*().
func (s *State) Validators() *ValidatorSet {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.validators
}

func (s *State) AccountBalance(accountId uint64) *AccountBalance {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	accBal := s.accountBalances.Get(accountId)
	if accBal == nil {
		return nil
	}
	return accBal.(*AccountBalance)
}
