package state

import (
	"bytes"
	"errors"
	"sync"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

var (
	ErrStateInvalidSequenceNumber = errors.New("Error State invalid sequence number")

	stateKey = []byte("stateKey")
)

type State struct {
	mtx        sync.Mutex
	db         db_.Db
	height     uint32 // Last known block height
	blockHash  []byte // Last known block hash
	commitTime time.Time
	accounts   merkle.Tree
	validators *ValidatorSet
}

func LoadState(db db_.Db) *State {
	s := &State{}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		s.height = uint32(0)
		s.commitTime = time.Unix(0, 0)      // XXX BOOTSTRAP
		s.blockHash = nil                   // XXX BOOTSTRAP
		s.accounts = merkle.NewIAVLTree(db) // XXX BOOTSTRAP
		s.validators = NewValidatorSet(nil) // XXX BOOTSTRAP
	} else {
		reader := bytes.NewReader(buf)
		var n int64
		var err error
		s.height = ReadUInt32(reader, &n, &err)
		s.commitTime = ReadTime(reader, &n, &err)
		s.blockHash = ReadByteSlice(reader, &n, &err)
		accountsMerkleRoot := ReadByteSlice(reader, &n, &err)
		s.accounts = merkle.NewIAVLTreeFromHash(db, accountsMerkleRoot)
		s.validators = NewValidatorSet(nil)
		for reader.Len() > 0 {
			validator := ReadValidator(reader, &n, &err)
			s.validators.Add(validator)
		}
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
	s.accounts.Save()
	var buf bytes.Buffer
	var n int64
	var err error
	WriteUInt32(&buf, s.height, &n, &err)
	WriteTime(&buf, commitTime, &n, &err)
	WriteByteSlice(&buf, s.blockHash, &n, &err)
	WriteByteSlice(&buf, s.accounts.Hash(), &n, &err)
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
		db:         s.db,
		height:     s.height,
		commitTime: s.commitTime,
		blockHash:  s.blockHash,
		accounts:   s.accounts.Copy(),
		validators: s.validators.Copy(),
	}
}

// If the tx is invalid, an error will be returned.
// Unlike CommitBlock(), state will not be altered.
func (s *State) CommitTx(tx Tx) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.commitTx(tx)
}

func (s *State) commitTx(tx Tx) error {
	/*
		// Get the signer's incr
		signerId := tx.Signature().SignerId
		if mem.state.AccountSequence(signerId) != tx.Sequence() {
			return ErrStateInvalidSequenceNumber
		}
	*/
	// TODO commit the tx
	panic("Implement CommitTx()")
	return nil
}

// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling Commit!
func (s *State) CommitBlock(b *Block) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Basic block validation.
	err := b.ValidateBasic(s.height, s.blockHash)
	if err != nil {
		return err
	}

	// Commit each tx
	for _, tx := range b.Data.Txs {
		err := s.commitTx(tx)
		if err != nil {
			return err
		}
	}

	// After all state has been mutated, finally increment validators.
	s.validators.IncrementAccum()
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

func (s *State) Account(accountId uint64) (*Account, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	idBytes, err := BasicCodec.Write(accountId)
	if err != nil {
		return nil, err
	}
	accountBytes := s.accounts.Get(idBytes)
	if accountBytes == nil {
		return nil, nil
	}
	n, err := int64(0), error(nil)
	account := ReadAccount(bytes.NewBuffer(accountBytes), &n, &err)
	return account, err
}
