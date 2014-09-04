package state

import (
	"bytes"
	"sync"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
)

var (
	stateKey = []byte("stateKey")
)

type State struct {
	mtx        sync.Mutex
	db         db_.Db
	height     uint32
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
		s.accounts = merkle.NewIAVLTree(db) // XXX BOOTSTRAP
		s.validators = NewValidatorSet(nil) // XXX BOOTSTRAP
	} else {
		reader := bytes.NewReader(buf)
		var n int64
		var err error
		s.height = ReadUInt32(reader, &n, &err)
		s.commitTime = ReadTime(reader, &n, &err)
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

func (s *State) Save() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.accounts.Save()
	var buf bytes.Buffer
	var n int64
	var err error
	WriteUInt32(&buf, s.height, &n, &err)
	WriteTime(&buf, s.commitTime, &n, &err)
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
		accounts:   s.accounts.Copy(),
		validators: s.validators.Copy(),
	}
}

func (s *State) CommitTx(tx *Tx) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	// TODO commit the tx
	panic("Implement CommitTx()")
	return nil
}

func (s *State) CommitBlock(b *Block, commitTime time.Time) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	// TODO commit the txs
	// XXX also increment validator accum.
	panic("Implement CommitBlock()")
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

func (s *State) Account(accountId uint64) *Account {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	// XXX: figure out a way to load an Account Binary type.
	return s.accounts.Get(accountId)
}
